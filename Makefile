.PHONY: format test test-e2e test-route-e2e test-real-chain demo inspect-demo devnet compose-devnet

GO_TEST_ENV = GOCACHE=/tmp/aegislink-gocache GOMODCACHE=/tmp/aegislink-gomodcache

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

demo:
	@echo "Running the live local AegisLink demo flow..."
	@echo "Proof path: Ethereum deposit -> AegisLink settlement -> routed packet -> destination execution -> async acknowledgement"
	@cd tests/e2e && $(GO_TEST_ENV) go test ./... -run 'TestFullBridgeLoopCanRouteDepositToCompletedOsmosisTransfer|TestRouteRelayerCanUseConfiguredAlternatePoolOnMockTarget'

inspect-demo:
	@echo "Inspecting the live local demo surfaces..."
	@echo "Inspection surfaces: /status /packets /executions /pools /balances /swaps"
	@cd tests/e2e && $(GO_TEST_ENV) go test ./... -run 'TestRouteRelayerCanUseConfiguredAlternatePoolOnMockTarget'

devnet:
	@echo "AegisLink shell:"
	@$(GO_TEST_ENV) go run ./chain/aegislink/cmd/aegislinkd
	@echo "Deterministic inbound proof:"
	@$(MAKE) test-e2e
	@echo "Route-focused proof:"
	@$(MAKE) test-route-e2e

compose-devnet:
	@docker compose --profile devnet up
