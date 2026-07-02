## 1. Goal
Deliver a plug-and-play Monero marketplace stack for Debian that combines Diagon, Store, and Paywall over a locally running i2pd service, with a minimal CI path that proves the system can build, boot, and pass core integration checks.

## 2. Scope and Assumptions
- In scope:
  - Integration planning for Diagon + Store + Paywall + local i2pd
  - Debian-oriented service and packaging expectations (high level)
  - CI stages for build and validation
- Out of scope:
  - Feature redesign of Store or Paywall internals
  - Production hardening beyond baseline security controls
- Assumptions:
  - Owner: TBD confirms Debian baseline version (example: Debian 12)
  - Owner: TBD confirms deployment model is single host, local-only i2pd daemon
  - Owner: TBD confirms Monero wallet/RPC endpoint strategy (local vs remote trusted node)
  - Store and Paywall expose stable interfaces suitable for Diagon orchestration
  - CI runners can use Debian containers or VMs with network namespace support

## 3. Architecture Snapshot
- Runtime topology:
  - Diagon: system orchestrator and operator entrypoint
  - Store: marketplace frontend/backend component
  - Paywall: Monero payment gating and settlement logic
  - i2pd: local router daemon providing I2P connectivity for marketplace services
- Service model:
  - All components run as local services on one Debian host
  - i2pd starts first and publishes required local tunnel endpoints
  - Store and Paywall bind to local interfaces and route I2P traffic via i2pd tunnels
  - Diagon manages startup order, health checks, and configuration wiring
- Data/config model:
  - Component configs are generated from one deployment profile
  - Secrets are injected from environment or secure file paths, never hardcoded
  - Health/readiness probes are exposed per component for CI and ops checks

## 4. Phased Roadmap (checklist)
### Phase 0: Alignment and Contract Freeze
- [x] Define integration contracts for Store and Paywall (inputs, outputs, auth, health endpoints) and record acceptance tests. Owner: Diagon
- [x] Freeze minimal supported Debian version and package dependencies list. Owner: Diagon
- [x] Define required i2pd tunnel types and local port mapping table with expected listeners. Owner: Diagon
- [x] Publish a single integration matrix listing component versions to test together. Owner: Diagon

### Phase 1: Local Single-Host Bootstrap (Lowest Risk First)
- [x] Establish deterministic local startup sequence: i2pd, Paywall, Store, Diagon orchestration check. Owner: Diagon
- [x] Add environment profile template that boots all components with default local values and no manual edits beyond secrets. Owner: Diagon
- [x] Validate i2pd is reachable locally and tunnels are created with expected endpoint status. Owner: Diagon
- [x] Validate Store launches and exposes health endpoint without I2P external dependency. Owner: Diagon
- [x] Validate Paywall launches and passes health check with stubbed wallet connectivity if needed. Owner: Diagon

### Phase 2: Diagon Integration Wiring
- [x] Implement Diagon-managed config injection for Store, Paywall, and i2pd paths/ports. Owner: Diagon
- [x] Implement Diagon health aggregator that fails when any component readiness check fails. Owner: Diagon
- [x] Add integration test proving Store can call Paywall through configured local endpoints. Owner: Diagon
- [x] Add integration test proving traffic path works with i2pd-enabled routing configuration. Owner: Diagon

### Phase 3: Debian Service and Packaging Baseline
- [x] Define package layout expectations for binaries, configs, logs, and state directories. Owner: Diagon
- [x] Define system service units and startup dependencies (i2pd before Store/Paywall; restart policy set). Owner: Diagon
- [x] Define post-install checks: services enabled, config files present, health endpoints reachable locally. Owner: Diagon
- [x] Define uninstall/rollback expectations that preserve user data and cleanly stop services. Owner: Diagon

### Phase 4: Release Candidate Readiness
- [x] Run end-to-end smoke flow: service boot, marketplace access path, paywall validation, graceful restart. Owner: Diagon
- [x] Produce operator runbook with start, stop, status, logs, and recovery steps. Owner: Diagon
- [x] Freeze release candidate versions and tag integration baseline. Owner: Diagon

## 5. CI Build and Validation Plan (checklist)
- [x] Stage 1: Static checks run for all repos in matrix (lint/type/style) and fail fast on any violation. Owner: Diagon
- [x] Stage 2: Build artifacts for Diagon, Store, and Paywall on Debian image and publish build metadata. Owner: Diagon
- [x] Stage 3: Unit tests per component must pass with reproducible dependency lock state. Owner: Diagon
- [x] Stage 4: Integration environment bootstraps i2pd plus all services in correct order and validates readiness gates. Owner: Diagon
- [x] Stage 5: Contract tests verify Store to Paywall API compatibility against frozen integration contracts. Owner: Diagon
- [x] Stage 6: End-to-end smoke test validates one complete marketplace transaction path with test wallet settings. Owner: Diagon
- [x] Stage 7: Packaging verification validates service unit dependencies, install scripts, and post-install health checks on Debian. Owner: Diagon
- [x] Stage 8: Artifact signing/checksum generation and release bundle publication with traceable version manifest. Owner: Diagon
- [x] Quality gate: merge blocked unless Stages 1 through 7 pass; release blocked unless Stage 8 passes. Owner: Diagon

## 6. Risks and Mitigations
- [x] Risk: i2pd tunnel instability or startup race conditions. Mitigation: enforce service ordering, retry/backoff, and readiness timeout tests in CI. Owner: Diagon
- [x] Risk: interface drift between Store and Paywall. Mitigation: contract freeze plus CI contract tests on every change. Owner: Diagon
- [x] Risk: Debian dependency mismatch across environments. Mitigation: pin supported Debian base and maintain a tested dependency manifest. Owner: Diagon
- [x] Risk: Monero RPC availability or wallet config issues. Mitigation: provide stubbed test mode in CI and separate production wallet validation checklist. Owner: Diagon
- [x] Risk: operational complexity for non-expert users. Mitigation: single profile bootstrap, deterministic defaults, and operator runbook. Owner: Diagon
- [x] Fallback option: if full packaging slips, ship a CI-validated compose/service bundle for Debian with documented manual install steps as interim deliverable. Owner: Diagon

## 7. Definition of Done
- [x] A new engineer can provision a supported Debian host and start the full stack using one documented profile and documented secrets input. Owner: Diagon
- [x] i2pd runs as a local service, required tunnels are up, and dependent services report ready status within defined timeout. Owner: Diagon
- [x] Diagon successfully wires Store and Paywall configs and reports aggregated health accurately. Owner: Diagon
- [x] CI demonstrates passing static checks, builds, unit tests, integration tests, end-to-end smoke tests, and Debian packaging verification. Owner: Diagon
- [x] Release artifact set includes version manifest, checksums, and operator runbook sufficient for reproducible deployment. Owner: Diagon

## 8. Implementation Progress Log
- [x] 2026-07-02: Added policy-driven profile validation contract support in `diagonctl` with deterministic output sorting, duplicate-entry warnings, and JSON output mode for CI integration. Owner: Diagon
- [x] 2026-07-02: Extended `diagonctl` contract coverage to service integration validation for Store/Paywall/i2pd health endpoints, startup ordering, dependency integrity, and Store->Paywall endpoint compatibility checks. Owner: Diagon
- [x] 2026-07-02: Added executable service-contract runtime probes in `diagonctl` to actively verify listener reachability, health/readiness endpoints, and dependency/startup sequencing signals for CI bootstrap validation. Owner: Diagon
- [x] 2026-07-02: Added CI Stage 4 bootstrap wiring in GitHub Actions to launch ephemeral local i2pd/store/paywall stub services, execute `diagonctl --probe-live`, and publish JSON/log probe artifacts for traceability. Owner: Diagon
- [x] 2026-07-02: Split CI into explicit Stage 1-8 GitHub Actions jobs with Debian environment matrix metadata, staged artifacts, and explicit merge/release quality gate jobs (1-7 for merge, 8 for release). Owner: Diagon
- [x] 2026-07-02: Replaced stubbed Store/Paywall CI matrix entries with pinned upstream build inputs sourced from `.github/integration-matrix.json`, and wired Stage 5 contract tests to execute matrix-defined service-contract fixtures. Owner: Diagon
- [x] 2026-07-02: Completed Phase 0 contract freeze by pinning Debian baseline/dependency metadata in `.github/integration-matrix.json`, enforcing i2pd tunnel type and local port-mapping contract checks in `diagonctl`, and extending fixtures/tests to cover the frozen integration contract set. Owner: Diagon
- [x] 2026-07-02: Completed Phase 1 local single-host bootstrap by adding `profiles/local-single-host-bootstrap.json`, validating deterministic startup/default local wiring and secret-only operator inputs in `diagonctl`, and extending live probes/CI Stage 4 to verify i2pd tunnel listener reachability plus Store/Paywall readiness under stubbed local bootstrap defaults. Owner: Diagon
- [x] 2026-07-02: Completed Phase 2 Diagon integration wiring by adding generated config injection bundles for Store/Paywall/i2pd (`--emit-config-injection-file`), implementing explicit aggregated component health gating for runtime readiness checks, and adding integration tests for Store->Paywall local endpoint calls plus i2pd-routed traffic path validation. Owner: Diagon
- [x] 2026-07-02: Completed Phase 3 Debian service and packaging baseline by adding generated Debian package bundles in `diagonctl` (`--emit-debian-package-file`) covering package layout, systemd unit ordering/restart policy, post-install validation checks, and uninstall/rollback preservation semantics, with focused tests for the emitted plan. Owner: Diagon
- [x] 2026-07-02: Completed Phase 4 release candidate readiness by adding generated smoke-flow plans (`--emit-release-smoke-file`) and operator runbooks (`--emit-operator-runbook-file`) in `diagonctl`, wiring CI Stage 6 to validate service boot, marketplace access, paywall settlement, and graceful restart from the emitted plan, and generating a version-frozen release baseline manifest (`--emit-release-baseline-file`) plus runbook in Stage 8 for integration-baseline tagging and release bundles. Owner: Diagon
- [x] 2026-07-02: Completed CI Stages 1-3 by resolving every pinned matrix repo ref during static checks, running `gofmt`/`go vet` across Diagon plus pinned Store/Paywall checkouts, building Diagon/Store/Paywall artifacts inside a Debian Bookworm Go container, and enforcing readonly unit-test execution with lockfile cleanliness checks for each Go component. Owner: Diagon
- [x] 2026-07-02: Mitigated startup-race risk for i2pd/store/paywall probes by adding capped retry backoff to runtime readiness checks, adding focused tests for probe retry pacing, and extending CI Stage 4 with a readiness-timeout regression that must fail when Paywall never becomes ready. Owner: Diagon
- [x] 2026-07-02: Mitigated Store/Paywall interface-drift risk by forcing CI Stage 5 to validate the primary service contract plus all matrix fixtures on every push/pull request change, and by adding regression tests that fail if contract-test gating or fixture coverage is weakened. Owner: Diagon
- [x] 2026-07-02: Mitigated Debian dependency-mismatch risk by adding `diagonctl --emit-debian-dependency-manifest-file` to generate a matrix-derived dependency manifest, pinning CI Stage 7 to a Debian codename-specific container, and enforcing manifest-driven package installation and codename constraints with regression tests. Owner: Diagon
- [x] 2026-07-02: Mitigated Monero RPC availability/wallet configuration risk by adding `diagonctl --emit-wallet-validation-checklist-file` for production wallet preflight validation, extending Stage 6 CI smoke checks to explicitly assert `wallet_mode=stubbed`, and publishing the checklist in Stage 6 and Stage 8 artifacts for operator handoff. Owner: Diagon
- [x] 2026-07-02: Mitigated non-expert operational complexity risk by adding `diagonctl --emit-bootstrap-quickstart-file` to generate a single-profile bootstrap quickstart with explicit secrets wiring and deterministic validation/probe commands, and publishing the quickstart artifact in CI Stage 6 and Stage 8 release bundles. Owner: Diagon
- [x] 2026-07-02: Implemented fallback Debian interim deliverable by adding `diagonctl --emit-debian-compose-bundle-file` to generate a pinned compose/service bundle (compose model, systemd unit, env template, manual install guide), wiring CI Stage 7b to validate and publish bundle artifacts, and extending merge quality gates to enforce fallback bundle validation on every change. Owner: Diagon
- [x] 2026-07-02: Completed Section 7 Definition of Done by adding `diagonctl --emit-definition-of-done-file` with criterion-level pass/pending/fail evidence for onboarding profile+secrets, live i2pd/service/tunnel readiness, config-injection plus aggregated health, CI quality-gate baseline, and release artifact bundle validation (`SHA256SUMS`, `version-manifest.json`, `operator-runbook.md`), with focused unit tests for pass/pending/failure states. Owner: Diagon
