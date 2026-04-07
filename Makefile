.PHONY: format test test-e2e test-route-e2e test-real-chain test-real-ibc demo inspect-demo real-demo inspect-real-demo devnet compose-devnet

GO_CACHE_ROOT ?= /tmp/aegislink-e2e-go-cache
GO_TEST_ENV = GOCACHE=$(GO_CACHE_ROOT)/gocache GOMODCACHE=$(GO_CACHE_ROOT)/gomodcache

format:
	@gofmt -w chain/aegislink relayer tests/e2e

test:
	@$(GO_TEST_ENV) go test ./chain/aegislink/...
	@cd contracts/ethereum && forge test --offline
	@$(GO_TEST_ENV) go test ./relayer/...

test-e2e:
	@cd tests/e2e && $(GO_TEST_ENV) go test ./...

test-route-e2e:
	@cd tests/e2e && $(GO_TEST_ENV) go test ./... -run 'TestOsmosisRoute|TestRouteRelayer|TestFullBridgeLoopCanRouteDepositToCompletedOsmosisTransfer'

test-real-chain:
	@cd tests/e2e && $(GO_TEST_ENV) go test ./... -run 'TestRealAegisLinkChain'

test-real-ibc:
	@cd tests/e2e && $(GO_TEST_ENV) go test ./... -run 'TestRealDestinationChainBootstrap|TestRealIBCRoute'

demo:
	@echo "Running the live local AegisLink demo flow..."
	@echo "Proof path: Ethereum deposit -> AegisLink settlement -> routed packet -> destination execution -> async acknowledgement"
	@cd tests/e2e && $(GO_TEST_ENV) go test ./... -run 'TestFullBridgeLoopCanRouteDepositToCompletedOsmosisTransfer|TestRouteRelayerCanUseConfiguredAlternatePoolOnMockTarget'

inspect-demo:
	@echo "Inspecting the live local demo surfaces..."
	@echo "Inspection surfaces: /status /metrics /packets /executions /pools /balances /swaps"
	@cd tests/e2e && $(GO_TEST_ENV) go test ./... -run 'TestRouteRelayerCanUseConfiguredAlternatePoolOnMockTarget'

real-demo:
	@echo "Running the real dual-runtime route demo..."
	@bash scripts/localnet/demo_real_ibc.sh demo

inspect-real-demo:
	@echo "Inspecting the real dual-runtime route path..."
	@bash scripts/localnet/demo_real_ibc.sh inspect

devnet:
	@echo "AegisLink shell:"
	@$(GO_TEST_ENV) go run ./chain/aegislink/cmd/aegislinkd
	@echo "Deterministic inbound proof:"
	@$(MAKE) test-e2e
	@echo "Route-focused proof:"
	@$(MAKE) test-route-e2e

compose-devnet:
	@docker compose --profile devnet up
