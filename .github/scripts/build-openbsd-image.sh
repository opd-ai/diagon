#!/usr/bin/env bash
# Builds a directly bootable/installable OpenBSD disk image for Diagon.
#
# This is the OpenBSD analog of build-debian-iso.sh. Where the Debian pipeline
# uses simple-cdd to bake an installer ISO from the shared profile, this script
# drives OpenBSD autoinstall(8) under QEMU using the shared config in
# profiles/openbsd/ and emits a raw disk image that can be booted directly or
# written to a target disk.
#
# Requires the following in the environment:
#   OPENBSD_VERSION  e.g. 7.5
#   OPENBSD_ARCH     e.g. amd64
#   OPENBSD_MIRROR   e.g. https://cdn.openbsd.org/pub/OpenBSD
#
# Requires these tools on PATH: qemu-system-x86_64, qemu-img, xorriso, expect,
# curl, python3, tar.
set -euo pipefail

: "${OPENBSD_VERSION:?OPENBSD_VERSION is required}"
: "${OPENBSD_ARCH:?OPENBSD_ARCH is required}"
: "${OPENBSD_MIRROR:?OPENBSD_MIRROR is required}"

ver="$OPENBSD_VERSION"
arch="$OPENBSD_ARCH"
mirror="$OPENBSD_MIRROR"
vshort="${ver/./}"           # 7.5 -> 75
http_port=8080

work="build/openbsd"
sets_dir="$work/http/pub/OpenBSD/${ver}/${arch}"
images_dir="$work/images"
rm -rf "$work"
mkdir -p "$sets_dir" "$images_dir"

echo "==> Assembling shared Diagon site set from profiles/openbsd"
site_stage="$work/site"
mkdir -p "$site_stage/var/diagon"
install -m 0755 profiles/openbsd/install.site "$site_stage/install.site"
install -m 0644 profiles/openbsd/pkg.list "$site_stage/var/diagon/pkg.list"
tar -C "$site_stage" -czf "$sets_dir/site${vshort}.tgz" .

echo "==> Downloading OpenBSD ${ver}/${arch} sets and installer media"
base_url="${mirror}/${ver}/${arch}"
for f in "bsd" "bsd.mp" "bsd.rd" "base${vshort}.tgz" "comp${vshort}.tgz"; do
	curl -fsSL -o "$sets_dir/$f" "$base_url/$f"
done
curl -fsSL -o "$sets_dir/index.txt" "$base_url/index.txt" || true

echo "==> Regenerating SHA256 over the local set mirror (includes site set)"
(
	cd "$sets_dir"
	files=$(ls -1 | grep -v '^SHA256$' || true)
	# shellcheck disable=SC2086
	sha256sum $files | sed -E 's/^([0-9a-f]+)  (.*)$/SHA256 (\2) = \1/' > SHA256
)

iso_orig="$work/install${vshort}.iso"
curl -fsSL -o "$iso_orig" "$base_url/install${vshort}.iso"

echo "==> Remastering installer ISO to use the serial console (com0)"
boot_conf="$work/boot.conf"
printf 'set tty com0\n' > "$boot_conf"
iso_serial="$work/install${vshort}-serial.iso"
xorriso -indev "$iso_orig" -outdev "$iso_serial" \
	-boot_image any replay \
	-map "$boot_conf" /etc/boot.conf

echo "==> Generating build-specific autoinstall response file"
conf="$work/http/install.conf"
cp profiles/openbsd/install.conf "$conf"
# The set location is the local mirror reached over QEMU user networking. The
# guest reaches the host HTTP server through a dedicated SLIRP guest address
# (10.0.2.2 is reserved as the gateway and cannot be used for guestfwd). Only
# the server host is build-specific; everything else lives in the shared config.
printf 'HTTP Server = 10.0.2.100\n' >> "$conf"

image="$images_dir/diagon-openbsd-${ver}-${arch}.img"
echo "==> Creating blank ${image} target disk"
qemu-img create -f raw "$image" 8G

echo "==> Serving set mirror + response file on 127.0.0.1:${http_port}"
python3 -m http.server "$http_port" --directory "$work/http" >/dev/null 2>&1 &
http_pid=$!
trap 'kill "$http_pid" 2>/dev/null || true' EXIT

# QEMU's guestfwd opens its forwarding socket to the host service eagerly at
# startup, so the mirror must already accept connections before QEMU launches.
echo "==> Waiting for local mirror to accept connections"
mirror_ready=0
for _ in $(seq 1 50); do
	if (exec 3<>"/dev/tcp/127.0.0.1/${http_port}") 2>/dev/null; then
		exec 3>&- 3<&-
		mirror_ready=1
		break
	fi
	sleep 0.2
done
if [[ "$mirror_ready" -ne 1 ]]; then
	echo "ERROR: local mirror on 127.0.0.1:${http_port} did not become ready" >&2
	exit 1
fi

accel="tcg"
if [[ -w /dev/kvm ]]; then
	accel="kvm:tcg"
fi

echo "==> Running unattended OpenBSD autoinstall under QEMU (accel=${accel})"
export QEMU_ISO="$iso_serial" QEMU_IMAGE="$image" QEMU_ACCEL="$accel" \
	QEMU_HTTP_PORT="$http_port"
expect <<'EXPECT'
set timeout 3600
set iso $env(QEMU_ISO)
set image $env(QEMU_IMAGE)
set accel $env(QEMU_ACCEL)
set port $env(QEMU_HTTP_PORT)

spawn qemu-system-x86_64 \
	-accel $accel \
	-m 2048 -smp 2 \
	-drive file=$image,format=raw,if=virtio \
	-cdrom $iso -boot d \
	-netdev user,id=n0,guestfwd=tcp:10.0.2.100:80-tcp:127.0.0.1:$port \
	-device virtio-net-pci,netdev=n0 \
	-nographic -serial mon:stdio -vga none

# Choose autoinstall at the initial installer prompt.
expect {
	-re {\(A\)utoinstall} { send "A\r" }
	timeout { puts "TIMEOUT: installer prompt"; exit 1 }
}

# Point autoinstall at the response file served from the host.
expect {
	-re {Response file location} { send "http://10.0.2.100/install.conf\r" }
	timeout { puts "TIMEOUT: response file prompt"; exit 1 }
}

# autoinstall now runs unattended from install.conf and reboots on success.
expect {
	-re {CONGRATULATIONS} { }
	-re {rebooting} { }
	-re {halt|Halting} { }
	timeout { puts "TIMEOUT: install did not complete"; exit 1 }
}

# Give the guest a moment to flush the reboot, then terminate QEMU.
send "\001x"
expect eof
EXPECT

echo "==> Wrote installable disk image: ${image}"
ls -1 "$images_dir"/*.img > "$work/image-list.txt"
(
	cd "$images_dir"
	sha256sum ./*.img > SHA256SUMS
)
