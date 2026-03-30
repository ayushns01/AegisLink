.PHONY: format test test-e2e devnet

format:
	@echo "Format targets wired; source files are not implemented yet."

test:
	@test -f chain/aegislink/app/app.go || { echo "Missing chain implementation: chain/aegislink/app/app.go"; exit 1; }
	@test -f relayer/cmd/bridge-relayer/main.go || { echo "Missing relayer implementation: relayer/cmd/bridge-relayer/main.go"; exit 1; }
	@test -f contracts/ethereum/BridgeGateway.sol || { echo "Missing Ethereum implementation: contracts/ethereum/BridgeGateway.sol"; exit 1; }
	@echo "Bootstrap wiring verified."

test-e2e:
	@echo "E2E targets wired; tests are not implemented yet."; exit 1

devnet:
	@echo "Devnet targets wired; services are not implemented yet."; exit 1
