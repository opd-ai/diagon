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
- [ ] Define integration contracts for Store and Paywall (inputs, outputs, auth, health endpoints) and record acceptance tests. Owner: TBD
- [ ] Freeze minimal supported Debian version and package dependencies list. Owner: TBD
- [ ] Define required i2pd tunnel types and local port mapping table with expected listeners. Owner: TBD
- [ ] Publish a single integration matrix listing component versions to test together. Owner: TBD

### Phase 1: Local Single-Host Bootstrap (Lowest Risk First)
- [ ] Establish deterministic local startup sequence: i2pd, Paywall, Store, Diagon orchestration check. Owner: TBD
- [ ] Add environment profile template that boots all components with default local values and no manual edits beyond secrets. Owner: TBD
- [ ] Validate i2pd is reachable locally and tunnels are created with expected endpoint status. Owner: TBD
- [ ] Validate Store launches and exposes health endpoint without I2P external dependency. Owner: TBD
- [ ] Validate Paywall launches and passes health check with stubbed wallet connectivity if needed. Owner: TBD

### Phase 2: Diagon Integration Wiring
- [ ] Implement Diagon-managed config injection for Store, Paywall, and i2pd paths/ports. Owner: TBD
- [ ] Implement Diagon health aggregator that fails when any component readiness check fails. Owner: TBD
- [ ] Add integration test proving Store can call Paywall through configured local endpoints. Owner: TBD
- [ ] Add integration test proving traffic path works with i2pd-enabled routing configuration. Owner: TBD

### Phase 3: Debian Service and Packaging Baseline
- [ ] Define package layout expectations for binaries, configs, logs, and state directories. Owner: TBD
- [ ] Define system service units and startup dependencies (i2pd before Store/Paywall; restart policy set). Owner: TBD
- [ ] Define post-install checks: services enabled, config files present, health endpoints reachable locally. Owner: TBD
- [ ] Define uninstall/rollback expectations that preserve user data and cleanly stop services. Owner: TBD

### Phase 4: Release Candidate Readiness
- [ ] Run end-to-end smoke flow: service boot, marketplace access path, paywall validation, graceful restart. Owner: TBD
- [ ] Produce operator runbook with start, stop, status, logs, and recovery steps. Owner: TBD
- [ ] Freeze release candidate versions and tag integration baseline. Owner: TBD

## 5. CI Build and Validation Plan (checklist)
- [ ] Stage 1: Static checks run for all repos in matrix (lint/type/style) and fail fast on any violation. Owner: TBD
- [ ] Stage 2: Build artifacts for Diagon, Store, and Paywall on Debian image and publish build metadata. Owner: TBD
- [ ] Stage 3: Unit tests per component must pass with reproducible dependency lock state. Owner: TBD
- [ ] Stage 4: Integration environment bootstraps i2pd plus all services in correct order and validates readiness gates. Owner: TBD
- [ ] Stage 5: Contract tests verify Store to Paywall API compatibility against frozen integration contracts. Owner: TBD
- [ ] Stage 6: End-to-end smoke test validates one complete marketplace transaction path with test wallet settings. Owner: TBD
- [ ] Stage 7: Packaging verification validates service unit dependencies, install scripts, and post-install health checks on Debian. Owner: TBD
- [ ] Stage 8: Artifact signing/checksum generation and release bundle publication with traceable version manifest. Owner: TBD
- [ ] Quality gate: merge blocked unless Stages 1 through 7 pass; release blocked unless Stage 8 passes. Owner: TBD

## 6. Risks and Mitigations
- [ ] Risk: i2pd tunnel instability or startup race conditions. Mitigation: enforce service ordering, retry/backoff, and readiness timeout tests in CI. Owner: TBD
- [ ] Risk: interface drift between Store and Paywall. Mitigation: contract freeze plus CI contract tests on every change. Owner: TBD
- [ ] Risk: Debian dependency mismatch across environments. Mitigation: pin supported Debian base and maintain a tested dependency manifest. Owner: TBD
- [ ] Risk: Monero RPC availability or wallet config issues. Mitigation: provide stubbed test mode in CI and separate production wallet validation checklist. Owner: TBD
- [ ] Risk: operational complexity for non-expert users. Mitigation: single profile bootstrap, deterministic defaults, and operator runbook. Owner: TBD
- [ ] Fallback option: if full packaging slips, ship a CI-validated compose/service bundle for Debian with documented manual install steps as interim deliverable. Owner: TBD

## 7. Definition of Done
- [ ] A new engineer can provision a supported Debian host and start the full stack using one documented profile and documented secrets input. Owner: TBD
- [ ] i2pd runs as a local service, required tunnels are up, and dependent services report ready status within defined timeout. Owner: TBD
- [ ] Diagon successfully wires Store and Paywall configs and reports aggregated health accurately. Owner: TBD
- [ ] CI demonstrates passing static checks, builds, unit tests, integration tests, end-to-end smoke tests, and Debian packaging verification. Owner: TBD
- [ ] Release artifact set includes version manifest, checksums, and operator runbook sufficient for reproducible deployment. Owner: TBD

## 8. Implementation Progress Log
- [x] 2026-07-02: Added policy-driven profile validation contract support in `diagonctl` with deterministic output sorting, duplicate-entry warnings, and JSON output mode for CI integration. Owner: Diagon
- [x] 2026-07-02: Extended `diagonctl` contract coverage to service integration validation for Store/Paywall/i2pd health endpoints, startup ordering, dependency integrity, and Store->Paywall endpoint compatibility checks. Owner: Diagon
- [x] 2026-07-02: Added executable service-contract runtime probes in `diagonctl` to actively verify listener reachability, health/readiness endpoints, and dependency/startup sequencing signals for CI bootstrap validation. Owner: Diagon
- [x] 2026-07-02: Added CI Stage 4 bootstrap wiring in GitHub Actions to launch ephemeral local i2pd/store/paywall stub services, execute `diagonctl --probe-live`, and publish JSON/log probe artifacts for traceability. Owner: Diagon
- [ ] Next: Split CI stages into explicit Stage 1-8 jobs with required-status quality gates (merge blocked on 1-7, release blocked on 8) and environment-specific matrix metadata. Owner: TBD
