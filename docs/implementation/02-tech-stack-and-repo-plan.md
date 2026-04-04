# AegisLink Tech Stack And Repo Plan

This document describes both the current technology choices already implemented in the repository and the planned upgrades that come next. The goal is to keep phase 1 buildable by a small team, with a clean path to phase 2 and a stronger Ethereum verifier later.

## Current Checkpoint

As of April 5, 2026, the repo has already implemented:

- the AegisLink persistent runtime and bridge-domain modules in Go
- the Ethereum gateway and verifier contracts in Solidity with Foundry tests
- the Go relayer pipeline with watchers, attestation collection, replay persistence, command-backed AegisLink integration, and RPC-backed Ethereum source and release paths
- the `ibcrouter` routing module with runtime query and tx surfaces for initiation, completion, failure, timeout, and refund handling
- a dedicated route-relayer plus mock target service for local routed-transfer handoff
- packet-shaped route delivery, asynchronous acknowledgement handling, and destination-side receive state with configurable multi-pool, fee-aware swap execution on the mock Osmosis target
- route-focused end-to-end tests, including a live Ethereum deposit that becomes a completed routed transfer record on AegisLink through that handoff

The main things still pending in this stack are a fuller Cosmos node runtime and a live local IBC or Osmosis environment beyond the current local target harness.

## Recommended Stack

### Core chain

- Language: Go
- Framework: Cosmos SDK
- Consensus: CometBFT
- Cross-chain messaging: IBC-Go
- Serialization: Protobuf with `buf`
- CLI tooling: Cobra and Viper

Why this stack:

- Cosmos SDK is Go-native, so the chain, keeper logic, and local tooling stay in one language.
- The Cosmos ecosystem already expects Go for chain modules and node code.
- Go makes it easier to share types and developer habits between the chain and the relayer.

### Ethereum side

- Language: Solidity
- Tooling: Foundry
- Libraries: OpenZeppelin
- Devnet: Anvil

Why this stack:

- Foundry gives fast tests and a small contract feedback loop.
- Solidity keeps the bridge contracts close to the standard Ethereum audit ecosystem.
- Anvil makes local integration testing predictable.

### Relayer and services

- Language: Go
- Current runtime: hybrid local runtime with RPC-backed Ethereum observation and release execution, command-backed AegisLink integration, and standard-library JSON fallbacks for fixture-driven paths
- Current config: environment-variable loader in Go
- Current route handoff: a dedicated Go route-relayer plus a lightweight HTTP target used to drive routed transfers during local dev and e2e tests
- Planned upgrade: `go-ethereum` for richer Ethereum client ergonomics beyond the current stdlib JSON-RPC path
- Planned upgrade: Cosmos client libraries or a fuller node interface for submitting bridge transactions directly into the chain
- Planned upgrade: richer config loading once the runtime moves beyond local fixtures

Why this stack:

- Keeping the relayer in Go avoids a second systems language.
- A single language reduces onboarding cost for an early-stage bridge project.
- The current hybrid runtime makes the bridge locally executable today while preserving fallback adapters for targeted tests.
- The route-relayer keeps outbound routing as a separate service boundary, which is closer to how a fuller IBC or downstream integration will eventually look.

### Local development and ops

- Containerization: Docker and Docker Compose
- Formatting: `gofmt`, `goimports`, `forge fmt`
- Linting: `golangci-lint`, `forge test`, `forge snapshot` where useful
- CI: GitHub Actions

## Local Development Setup

The target local setup should boot three things:

1. A local Ethereum devnet.
2. an AegisLink local node.
3. The relayer connected to both.

The current checkpoint is already past the first local integration target:

- the relayer can run against the persistent AegisLink runtime
- the relayer can observe live Ethereum deposits and execute live Ethereum releases against Anvil
- the contracts, runtime logic, relayer, and e2e loop all have passing test suites
- the route lifecycle to an Osmosis-style destination is now implemented on the AegisLink runtime side
- a local route-relayer and mock target can drive routed-transfer completion and recovery
- the next milestones are fuller Cosmos runtime realism and a live local IBC or Osmosis routing environment

Recommended developer workflow:

- `make bootstrap` installs or checks toolchain dependencies.
- `make devnet` starts the full local stack.
- `make test` runs the fast unit and contract tests.
- `make test-e2e` runs the bridge round-trip scenario.

## Proposed Repo Layout

The repo should be a monorepo so the shared protocol, chain logic, contracts, and relayer stay aligned.

Expected top-level structure:

- `chain/aegislink/cmd/aegislinkd/main.go`
- `chain/aegislink/app/app.go`
- `chain/aegislink/app/config.go`
- `chain/aegislink/x/bridge/module.go`
- `chain/aegislink/x/bridge/keeper/keeper.go`
- `chain/aegislink/x/bridge/keeper/keeper_test.go`
- `chain/aegislink/x/registry/module.go`
- `chain/aegislink/x/registry/keeper/keeper.go`
- `chain/aegislink/x/registry/keeper/keeper_test.go`
- `chain/aegislink/x/limits/module.go`
- `chain/aegislink/x/limits/keeper/keeper.go`
- `chain/aegislink/x/pauser/module.go`
- `chain/aegislink/x/pauser/keeper/keeper.go`
- `chain/aegislink/x/ibcrouter/module.go`
- `chain/aegislink/x/ibcrouter/keeper/keeper.go`
- `proto/aegislink/bridge/v1/bridge.proto`
- `proto/aegislink/registry/v1/registry.proto`
- `proto/aegislink/limits/v1/limits.proto`
- `contracts/ethereum/BridgeGateway.sol`
- `contracts/ethereum/BridgeVerifier.sol`
- `contracts/ethereum/test/BridgeGateway.t.sol`
- `contracts/ethereum/script/Deploy.s.sol`
- `relayer/cmd/bridge-relayer/main.go`
- `relayer/cmd/route-relayer/main.go`
- `relayer/cmd/mock-osmosis-target/main.go`
- `relayer/internal/attestations/collector.go`
- `relayer/internal/attestations/collector_test.go`
- `relayer/internal/attestations/file_source.go`
- `relayer/internal/attestations/file_source_test.go`
- `relayer/internal/config/config.go`
- `relayer/internal/config/config_test.go`
- `relayer/internal/config/route_config.go`
- `relayer/internal/config/route_config_test.go`
- `relayer/internal/evm/client.go`
- `relayer/internal/evm/client_test.go`
- `relayer/internal/evm/watcher.go`
- `relayer/internal/evm/watcher_test.go`
- `relayer/internal/evm/file_runtime.go`
- `relayer/internal/evm/file_runtime_test.go`
- `relayer/internal/cosmos/client.go`
- `relayer/internal/cosmos/client_test.go`
- `relayer/internal/cosmos/watcher.go`
- `relayer/internal/cosmos/watcher_test.go`
- `relayer/internal/cosmos/file_runtime.go`
- `relayer/internal/cosmos/file_runtime_test.go`
- `relayer/internal/replay/store.go`
- `relayer/internal/replay/store_test.go`
- `relayer/internal/pipeline/pipeline.go`
- `relayer/internal/pipeline/pipeline_test.go`
- `relayer/internal/route/relay.go`
- `relayer/internal/route/relay_test.go`
- `relayer/internal/route/command_runtime.go`
- `relayer/internal/route/command_runtime_test.go`
- `relayer/internal/route/http_target.go`
- `relayer/internal/route/http_target_test.go`
- `relayer/internal/route/mock_target.go`
- `tests/e2e/bridge_roundtrip_test.go`
- `tests/e2e/localnet_test.go`
- `README.md`
- `docs/foundations/01-bridge-basics.md`
- `docs/foundations/02-eth-cosmos-primer.md`
- `docs/security-model.md`
- `docs/observability.md`
- `docs/runbooks/pause-and-recovery.md`
- `docs/runbooks/upgrade-and-rollback.md`
- `docs/implementation/01-step-by-step-roadmap.md`
- `docs/implementation/02-tech-stack-and-repo-plan.md`
- `docs/superpowers/specs/2026-03-28-eth-cosmos-aegislink-design.md`
- `docs/superpowers/plans/2026-03-28-eth-cosmos-aegislink-implementation.md`
- `Makefile`
- `go.work`
- `buf.yaml`
- `buf.gen.yaml`
- `foundry.toml`
- `docker-compose.yml`
- `.gitignore`

## Testing Strategy

Use a layered test stack so each failure mode is caught at the cheapest level possible.

### Fast tests

- Go unit tests for keepers, message validation, replay protection, and relayer state machines.
- Solidity tests for contract logic and signature verification.
- These should run on every change.

### Integration tests

- Local chain tests that spin up a bridge zone node and send real transactions.
- Contract-to-relayer tests using Anvil and a mocked Cosmos endpoint.
- These should run before merge.

### End-to-end tests

- A full localnet that moves an asset from Ethereum to the bridge zone and back again.
- A second scenario that routes a supported asset from the bridge zone into the AegisLink-controlled Osmosis-style transfer lifecycle today, then through a route-relayer into a local target, and later into a fuller IBC or Osmosis environment.
- These should run in CI on a slower schedule or before release.

### Security tests

- Replay attack tests.
- Pause and unpause tests.
- Rate-limit boundary tests.
- Asset registry negative tests.

## What To Build First

Build in this order to reduce rework:

1. Repo scaffolding and build tooling.
2. Protobuf definitions and shared message formats.
3. Bridge zone core modules for registry, pause controls, replay protection, and rate limits.
4. Ethereum bridge contracts and contract tests.
5. Relayer event ingestion and Cosmos submission.
6. Localnet and end-to-end bridge flow.
7. IBC routing to Osmosis.
8. Observability, runbooks, and hardening.

The first milestone should prove one narrow happy path end to end before adding more asset types or more routes.

Current progress against that order:

- completed: steps 1 through 6 and the route-relayer-driven local slices of step 7
- next: deepen step 7 into a fuller local IBC or Osmosis harness, with fuller Cosmos runtime realism as a valuable parallel hardening track
