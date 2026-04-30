# AegisLink Neutron Testnet Destination — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Neutron testnet (`pion-1`) as a second live IBC destination alongside Osmosis testnet, so a user can bridge Sepolia ETH to a `neutron1...` address from the same frontend.

**Architecture:** The `RouteProfile` keeper already supports multiple routes with no changes. The live Sepolia→Cosmos flow is driven by `public-bridge-relayer` which uses `autodelivery.RlyFlusher` to flush IBC packets. `RlyFlusher` currently holds a single path name; it becomes a route-keyed map. The frontend's hardcoded `routeId` becomes per-destination. The backend script registers two routes and opens two `rly` paths.

**Tech Stack:** Go 1.22+, TypeScript/React (Vitest + Testing Library), Bash

---

## File Map

| File | Action | What changes |
|---|---|---|
| `relayer/internal/autodelivery/coordinator.go` | Modify | `Flusher` interface: add `routeID` param to `Flush` |
| `relayer/internal/autodelivery/coordinator_test.go` | Modify | Update `stubFlusher`, fix assertions |
| `relayer/internal/autodelivery/runtime.go` | Modify | `RlyFlusher`: `PathName→PathByRoute+DefaultPath`; `Flush` signature |
| `relayer/internal/autodelivery/runtime_test.go` | Modify | Add `RlyFlusher` path-lookup tests |
| `relayer/cmd/public-bridge-relayer/main.go` | Modify | Add `AutoDeliveryPathByRoute`; add `parseRlyPathMap`; wire new flusher |
| `relayer/cmd/public-bridge-relayer/main_test.go` | Modify | Add `parseRlyPathMap` tests |
| `web/src/features/bridge/TransferPage.tsx` | Modify | Add `routeId` to `Destination`; add Neutron entry; dynamic `routeId` on submit; dynamic CTA label |
| `web/src/features/bridge/bridge.test.tsx` | Modify | Add Neutron-enabled test; update `routeId` assertion; update CTA label assertions |
| `scripts/testnet/start_public_bridge_backend.sh` | Modify | Accept path arg in `run_relayer_link_with_retry`; add Neutron chain to `write_rly_config`; add Neutron key/path/route bootstrap; export `AEGISLINK_RELAYER_RLY_PATH_MAP` |
| `deploy/testnet/ibc/neutron-wallet-delivery.example.json` | Create | Neutron route manifest (example) |
| `deploy/testnet/ibc/rly/neutron-testnet.chain.example.json` | Create | `rly` chain metadata for `pion-1` |
| `.env.public-ibc.neutron.local.example` | Create | Neutron testnet env overrides |

---

## Task 1: Update `stubFlusher` in coordinator tests

**Files:**
- Modify: `relayer/internal/autodelivery/coordinator_test.go`

- [ ] **Step 1: Replace `stubFlusher` with two-argument version**

In `relayer/internal/autodelivery/coordinator_test.go`, replace the `stubFlusher` struct and its `Flush` method (lines 168–176):

```go
type stubFlusher struct {
	calls []struct{ routeID, channelID string }
	err   error
}

func (s *stubFlusher) Flush(_ context.Context, routeID, channelID string) error {
	s.calls = append(s.calls, struct{ routeID, channelID string }{routeID, channelID})
	return s.err
}
```

- [ ] **Step 2: Fix the flush assertion in `TestCoordinatorRunOnceInitiatesAndFlushesWhenClaimedIntentIsReady`**

Replace (line 55–57):
```go
if len(flusher.channels) != 1 || flusher.channels[0] != "channel-0" {
    t.Fatalf("expected one flush for channel-0, got %v", flusher.channels)
}
```
with:
```go
if len(flusher.calls) != 1 || flusher.calls[0].channelID != "channel-0" || flusher.calls[0].routeID != "osmosis-public-wallet" {
    t.Fatalf("expected one flush for channel-0 with osmosis route, got %v", flusher.calls)
}
```

- [ ] **Step 3: Fix `TestCoordinatorRunOnceWaitsWhileSepoliaConfirmationIsPending` (uses `flusher.channels`)**

Replace (line 95–97):
```go
if len(flusher.channels) != 0 {
    t.Fatalf("expected no flushes, got %v", flusher.channels)
}
```
with:
```go
if len(flusher.calls) != 0 {
    t.Fatalf("expected no flushes, got %v", flusher.calls)
}
```

- [ ] **Step 4: Verify the file no longer references `flusher.channels`**

```bash
grep -n "flusher\.channels" relayer/internal/autodelivery/coordinator_test.go
```
Expected: no output.

---

## Task 2: Update `Flusher` interface and `coordinator.go` call sites

**Files:**
- Modify: `relayer/internal/autodelivery/coordinator.go`

- [ ] **Step 1: Update the `Flusher` interface**

In `relayer/internal/autodelivery/coordinator.go`, replace (line 32–34):
```go
type Flusher interface {
	Flush(ctx context.Context, channelID string) error
}
```
with:
```go
type Flusher interface {
	Flush(ctx context.Context, routeID, channelID string) error
}
```

- [ ] **Step 2: Update the `aegislink_processing` flush call in `RunOnce`**

In `RunOnce`, the first `Flush` call (inside `case "aegislink_processing"`) currently reads:
```go
if err := c.flusher.Flush(ctx, transfer.ChannelID); err != nil {
```
Replace with:
```go
if err := c.flusher.Flush(ctx, intent.RouteID, transfer.ChannelID); err != nil {
```

- [ ] **Step 3: Update the `osmosis_pending` flush call in `RunOnce`**

The second `Flush` call (inside `case "osmosis_pending"`) currently reads:
```go
if err := c.flusher.Flush(ctx, channelID); err != nil {
```
Replace with:
```go
if err := c.flusher.Flush(ctx, intent.RouteID, channelID); err != nil {
```

- [ ] **Step 4: Run coordinator tests**

```bash
go test ./relayer/internal/autodelivery/... -run TestCoordinator -v
```
Expected: all three `TestCoordinator*` tests PASS.

- [ ] **Step 5: Commit**

```bash
git add relayer/internal/autodelivery/coordinator.go relayer/internal/autodelivery/coordinator_test.go
git commit -m "feat(autodelivery): route-aware Flusher interface — Flush now takes routeID"
```

---

## Task 3: Update `RlyFlusher` and add path-lookup tests

**Files:**
- Modify: `relayer/internal/autodelivery/runtime.go`
- Modify: `relayer/internal/autodelivery/runtime_test.go`

- [ ] **Step 1: Write failing `RlyFlusher` path-lookup tests**

Append to `relayer/internal/autodelivery/runtime_test.go`:

```go
func TestRlyFlusherSelectsPathByRouteID(t *testing.T) {
	t.Parallel()

	flusher := RlyFlusher{
		Command: "echo",
		PathByRoute: map[string]string{
			"osmosis-public-wallet": "live-osmo-path",
			"neutron-public-wallet": "live-ntrn-path",
		},
		DefaultPath: "fallback-path",
		Home:        "/tmp/rly-home",
	}

	resolved := flusher.resolvePath("neutron-public-wallet")
	if resolved != "live-ntrn-path" {
		t.Fatalf("expected live-ntrn-path, got %q", resolved)
	}

	resolved = flusher.resolvePath("osmosis-public-wallet")
	if resolved != "live-osmo-path" {
		t.Fatalf("expected live-osmo-path, got %q", resolved)
	}
}

func TestRlyFlusherFallsBackToDefaultPathWhenRouteNotInMap(t *testing.T) {
	t.Parallel()

	flusher := RlyFlusher{
		Command:     "echo",
		PathByRoute: map[string]string{"other-route": "other-path"},
		DefaultPath: "fallback-path",
		Home:        "/tmp/rly-home",
	}

	resolved := flusher.resolvePath("unknown-route")
	if resolved != "fallback-path" {
		t.Fatalf("expected fallback-path, got %q", resolved)
	}
}

func TestRlyFlusherReturnsEmptyWhenNoPathAvailable(t *testing.T) {
	t.Parallel()

	flusher := RlyFlusher{
		Command:     "echo",
		PathByRoute: map[string]string{},
		DefaultPath: "",
		Home:        "/tmp/rly-home",
	}

	resolved := flusher.resolvePath("any-route")
	if resolved != "" {
		t.Fatalf("expected empty path, got %q", resolved)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./relayer/internal/autodelivery/... -run TestRlyFlusher -v
```
Expected: compile error — `RlyFlusher` has no `resolvePath` method and still has old struct fields.

- [ ] **Step 3: Update `RlyFlusher` struct and implement `resolvePath` + new `Flush`**

In `relayer/internal/autodelivery/runtime.go`, replace the `RlyFlusher` struct and `Flush` method (starting at `type RlyFlusher struct`):

```go
type RlyFlusher struct {
	Command     string
	PathByRoute map[string]string // routeID → rly path name
	DefaultPath string            // fallback when routeID not in map
	Home        string
}

func (f RlyFlusher) resolvePath(routeID string) string {
	if routeID != "" && f.PathByRoute != nil {
		if path, ok := f.PathByRoute[routeID]; ok && strings.TrimSpace(path) != "" {
			return strings.TrimSpace(path)
		}
	}
	return strings.TrimSpace(f.DefaultPath)
}

func (f RlyFlusher) Flush(ctx context.Context, routeID, channelID string) error {
	command := strings.TrimSpace(f.Command)
	if command == "" {
		command = "./bin/relayer"
	}
	pathName := f.resolvePath(routeID)
	if pathName == "" {
		return fmt.Errorf("missing relayer path for auto delivery flush (route: %q)", routeID)
	}
	home := strings.TrimSpace(f.Home)
	if home == "" {
		return fmt.Errorf("missing relayer home for auto delivery flush")
	}
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return fmt.Errorf("missing channel id for auto delivery flush")
	}

	cmd := exec.CommandContext(
		ctx,
		command,
		"transact",
		"flush",
		pathName,
		channelID,
		"--home",
		home,
		"--debug",
		"--log-level",
		"debug",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, message)
	}
	return nil
}
```

- [ ] **Step 4: Run all autodelivery tests**

```bash
go test ./relayer/internal/autodelivery/... -v
```
Expected: all tests PASS including the three new `TestRlyFlusher*` tests.

- [ ] **Step 5: Commit**

```bash
git add relayer/internal/autodelivery/runtime.go relayer/internal/autodelivery/runtime_test.go
git commit -m "feat(autodelivery): RlyFlusher dispatches by routeID via PathByRoute map"
```

---

## Task 4: Wire `PathByRoute` into `public-bridge-relayer`

**Files:**
- Modify: `relayer/cmd/public-bridge-relayer/main.go`

- [ ] **Step 1: Add `AutoDeliveryPathByRoute` field to `publicBridgeConfig`**

In `relayer/cmd/public-bridge-relayer/main.go`, in the `publicBridgeConfig` struct, add one field after `AutoDeliveryPathName`:

```go
AutoDeliveryPathName        string
AutoDeliveryPathByRoute     map[string]string
```

- [ ] **Step 2: Add `parseRlyPathMap` helper**

Add this function anywhere in `main.go` (e.g. near `buildPublicBridgeConfig`):

```go
// parseRlyPathMap parses "routeID:pathName,routeID:pathName" into a map.
func parseRlyPathMap(raw string) map[string]string {
	result := make(map[string]string)
	for _, entry := range strings.Split(strings.TrimSpace(raw), ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		routeID, pathName, ok := strings.Cut(entry, ":")
		if !ok {
			continue
		}
		routeID = strings.TrimSpace(routeID)
		pathName = strings.TrimSpace(pathName)
		if routeID != "" && pathName != "" {
			result[routeID] = pathName
		}
	}
	return result
}
```

- [ ] **Step 3: Read `AEGISLINK_RELAYER_RLY_PATH_MAP` in `buildPublicBridgeConfig`**

In `buildPublicBridgeConfig`, after the line that sets `AutoDeliveryPathName` (line 189):

```go
AutoDeliveryPathName:        strings.TrimSpace(os.Getenv("AEGISLINK_RELAYER_RLY_PATH_NAME")),
```

Add:

```go
AutoDeliveryPathByRoute:     parseRlyPathMap(os.Getenv("AEGISLINK_RELAYER_RLY_PATH_MAP")),
```

- [ ] **Step 4: Wire `PathByRoute` into `buildAutoDeliveryCoordinator`**

In `buildAutoDeliveryCoordinator`, replace the `autodelivery.RlyFlusher{...}` block (around line 418):

```go
autodelivery.RlyFlusher{
    Command:  cfg.AutoDeliveryRelayerCommand,
    PathName: cfg.AutoDeliveryPathName,
    Home:     cfg.AutoDeliveryRelayerHome,
},
```

with:

```go
autodelivery.RlyFlusher{
    Command:     cfg.AutoDeliveryRelayerCommand,
    PathByRoute: cfg.AutoDeliveryPathByRoute,
    DefaultPath: cfg.AutoDeliveryPathName,
    Home:        cfg.AutoDeliveryRelayerHome,
},
```

- [ ] **Step 5: Compile check**

```bash
go build ./relayer/cmd/public-bridge-relayer/...
```
Expected: builds with no errors.

- [ ] **Step 6: Run relayer tests**

```bash
go test ./relayer/... -v 2>&1 | tail -30
```
Expected: all tests PASS.

---

## Task 5: Add `parseRlyPathMap` tests

**Files:**
- Modify: `relayer/cmd/public-bridge-relayer/main_test.go`

- [ ] **Step 1: Append path-map parsing tests**

Add to `relayer/cmd/public-bridge-relayer/main_test.go`:

```go
func TestParseRlyPathMapEmptyStringReturnsEmptyMap(t *testing.T) {
	got := parseRlyPathMap("")
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}

func TestParseRlyPathMapSingleEntry(t *testing.T) {
	got := parseRlyPathMap("osmosis-public-wallet:live-osmo-path")
	if got["osmosis-public-wallet"] != "live-osmo-path" {
		t.Fatalf("expected live-osmo-path, got %v", got)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
}

func TestParseRlyPathMapTwoEntries(t *testing.T) {
	got := parseRlyPathMap("osmosis-public-wallet:live-osmo-path,neutron-public-wallet:live-ntrn-path")
	if got["osmosis-public-wallet"] != "live-osmo-path" {
		t.Fatalf("expected live-osmo-path for osmosis, got %v", got)
	}
	if got["neutron-public-wallet"] != "live-ntrn-path" {
		t.Fatalf("expected live-ntrn-path for neutron, got %v", got)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
}

func TestParseRlyPathMapIgnoresMalformedEntries(t *testing.T) {
	got := parseRlyPathMap("osmosis-public-wallet:live-osmo-path,bad-entry-no-colon,neutron-public-wallet:live-ntrn-path")
	if len(got) != 2 {
		t.Fatalf("expected 2 valid entries, got %d: %v", len(got), got)
	}
}

func TestParseRlyPathMapIgnoresWhitespace(t *testing.T) {
	got := parseRlyPathMap("  osmosis-public-wallet : live-osmo-path , neutron-public-wallet : live-ntrn-path  ")
	if got["osmosis-public-wallet"] != "live-osmo-path" {
		t.Fatalf("expected trimmed value, got %v", got)
	}
	if got["neutron-public-wallet"] != "live-ntrn-path" {
		t.Fatalf("expected trimmed value, got %v", got)
	}
}
```

- [ ] **Step 2: Run new tests**

```bash
go test ./relayer/cmd/public-bridge-relayer/... -run TestParseRlyPathMap -v
```
Expected: all five `TestParseRlyPathMap*` tests PASS.

- [ ] **Step 3: Run full relayer test suite**

```bash
go test ./relayer/... 2>&1 | tail -20
```
Expected: all packages PASS.

- [ ] **Step 4: Commit**

```bash
git add relayer/cmd/public-bridge-relayer/main.go relayer/cmd/public-bridge-relayer/main_test.go
git commit -m "feat(relayer): parse AEGISLINK_RELAYER_RLY_PATH_MAP for multi-destination flush dispatch"
```

---

## Task 6: Add Neutron tests to `bridge.test.tsx`

**Files:**
- Modify: `web/src/features/bridge/bridge.test.tsx`

- [ ] **Step 1: Add Neutron-enabled test**

Inside the `describe("TransferPage", ...)` block, after the last `it(...)`, add:

```ts
it("shows Neutron testnet as a live enabled destination", async () => {
  seedConnectedWallet();
  const user = userEvent.setup();
  render(<TransferPage />);

  await user.click(
    screen.getByRole("button", {
      name: /destination chain: osmosis testnet \(osmo\)/i,
    }),
  );

  const neutronItem = screen.getByRole("menuitem", {
    name: /neutron testnet \(ntrn\)/i,
  });
  expect(neutronItem).toBeInTheDocument();
  expect(neutronItem).not.toBeDisabled();
  expect(within(neutronItem).getByText(/live/i)).toBeInTheDocument();
});

it("switches to Neutron testnet and validates neutron1 recipient prefix", async () => {
  seedConnectedWallet();
  const user = userEvent.setup();
  render(<TransferPage />);

  await user.click(
    screen.getByRole("button", {
      name: /destination chain: osmosis testnet \(osmo\)/i,
    }),
  );
  await user.click(
    screen.getByRole("menuitem", { name: /neutron testnet \(ntrn\)/i }),
  );

  const recipientInput = screen.getByLabelText(/recipient/i);
  await user.clear(recipientInput);
  await user.type(recipientInput, "osmo1shouldfailneutronprefix");

  expect(
    screen.getByText(/enter a valid neutron1 recipient/i),
  ).toBeInTheDocument();

  await user.clear(recipientInput);
  await user.type(
    recipientInput,
    "neutron1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
  );

  expect(
    screen.queryByText(/enter a valid neutron1 recipient/i),
  ).not.toBeInTheDocument();
});

it("registers delivery intent with neutron-public-wallet routeId when Neutron is selected", async () => {
  seedConnectedWallet();
  submitEthDepositMock.mockResolvedValue(
    "0x422d075a86656b27694780b3ad553abee1dded6f3fb5bfa805137a3da64f30b8",
  );
  registerBridgeDeliveryIntentMock.mockResolvedValue(undefined);

  const user = userEvent.setup();
  render(<TransferPage />);

  // Switch to Neutron.
  await user.click(
    screen.getByRole("button", {
      name: /destination chain: osmosis testnet \(osmo\)/i,
    }),
  );
  await user.click(
    screen.getByRole("menuitem", { name: /neutron testnet \(ntrn\)/i }),
  );

  // Enter a valid neutron1 recipient.
  await user.clear(screen.getByLabelText(/recipient/i));
  await user.type(
    screen.getByLabelText(/recipient/i),
    "neutron1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
  );

  await user.click(
    screen.getByRole("button", { name: /bridge to neutron testnet/i }),
  );

  await waitFor(() => {
    expect(registerBridgeDeliveryIntentMock).toHaveBeenCalledWith(
      expect.objectContaining({
        routeId: "neutron-public-wallet",
        receiver: "neutron1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
      }),
    );
  });
});
```

- [ ] **Step 2: Run the new tests — expect them to fail**

```bash
cd web && npm test -- --reporter=verbose 2>&1 | grep -E "PASS|FAIL|neutron"
```
Expected: the three new Neutron tests FAIL (Neutron not in destinations yet).

---

## Task 7: Update `TransferPage.tsx`

**Files:**
- Modify: `web/src/features/bridge/TransferPage.tsx`

- [ ] **Step 1: Add `routeId` to the `Destination` type**

Replace (around line 13):
```ts
type Destination = {
  id: string;
  label: string;
  symbol: string;
  helper: string;
  enabled: boolean;
  prefix: string;
};
```
with:
```ts
type Destination = {
  id: string;
  label: string;
  symbol: string;
  helper: string;
  enabled: boolean;
  prefix: string;
  routeId: string;
};
```

- [ ] **Step 2: Add `routeId` to the Osmosis testnet entry and add the Neutron testnet entry**

Replace the Osmosis testnet entry (around line 22):
```ts
{
  id: "osmosis-testnet-osmo",
  label: "Osmosis Testnet (OSMO)",
  symbol: "OSMO",
  helper: "Live route available now",
  enabled: true,
  prefix: "osmo1",
},
```
with:
```ts
{
  id: "osmosis-testnet-osmo",
  label: "Osmosis Testnet (OSMO)",
  symbol: "OSMO",
  helper: "Live route available now",
  enabled: true,
  prefix: "osmo1",
  routeId: "osmosis-public-wallet",
},
{
  id: "neutron-testnet-ntrn",
  label: "Neutron Testnet (NTRN)",
  symbol: "NTRN",
  helper: "Live route available now",
  enabled: true,
  prefix: "neutron1",
  routeId: "neutron-public-wallet",
},
```

- [ ] **Step 3: Add `routeId` to all remaining disabled destination entries**

Each disabled entry needs `routeId: ""` (they are never submitted). Replace each disabled entry to add the field. For example the Osmosis mainnet entry becomes:

```ts
{
  id: "osmosis-mainnet-osmo",
  label: "Osmosis Mainnet (OSMO)",
  symbol: "OSMO",
  helper: "Coming soon",
  enabled: false,
  prefix: "osmo1",
  routeId: "",
},
```

Apply `routeId: ""` to all remaining disabled entries: Celestia mainnet, Celestia Mocha testnet, Injective mainnet, Injective testnet, dYdX mainnet, dYdX testnet, Akash mainnet, Akash sandbox.

- [ ] **Step 4: Replace the hardcoded `routeId` in `handleSubmit`**

In `handleSubmit`, replace (around line 162):
```ts
routeId: "osmosis-public-wallet",
```
with:
```ts
routeId: destination.routeId,
```

- [ ] **Step 5: Update the CTA button label**

Replace (around line 325):
```tsx
{isSubmitting ? "Opening Bridge Tunnel..." : "Bridge to Osmosis"}
```
with:
```tsx
{isSubmitting ? "Opening Bridge Tunnel..." : `Bridge to ${destination.label}`}
```

- [ ] **Step 6: Remove the "Osmosis only" caveat from the recipient helper text**

Replace (around line 308):
```tsx
<span className="field-helper">
  Recipient must match the selected chain prefix. Current route support
  is live for Osmosis only.
</span>
```
with:
```tsx
<span className="field-helper">
  Recipient must match the selected chain prefix.
</span>
```

- [ ] **Step 7: Run all frontend tests**

```bash
cd web && npm test -- --reporter=verbose 2>&1 | tail -40
```
Expected: all three new Neutron tests PASS. Check for any regressions in existing tests.

- [ ] **Step 8: Fix any regressions in existing tests**

The CTA button label changed from `"Bridge to Osmosis"` to `"Bridge to Osmosis Testnet (OSMO)"`. The existing test selector `{ name: /bridge to osmosis/i }` still matches because `"Osmosis"` is a substring — verify by running:

```bash
cd web && npm test -- --reporter=verbose 2>&1 | grep -E "FAIL|✗|×"
```
Expected: no failures. If you see failures on the CTA button selector, update those selectors to use `/bridge to osmosis testnet/i`.

- [ ] **Step 9: Commit**

```bash
git add web/src/features/bridge/TransferPage.tsx web/src/features/bridge/bridge.test.tsx
git commit -m "feat(web): add Neutron testnet destination with live route and neutron1 prefix validation"
```

---

## Task 8: Add new config files

**Files:**
- Create: `deploy/testnet/ibc/neutron-wallet-delivery.example.json`
- Create: `deploy/testnet/ibc/rly/neutron-testnet.chain.example.json`
- Create: `.env.public-ibc.neutron.local.example`

- [ ] **Step 1: Create `neutron-wallet-delivery.example.json`**

```json
{
  "enabled": false,
  "source_chain_id": "aegislink-public-testnet-1",
  "destination_chain_id": "pion-1",
  "provider": "hermes",
  "wallet_prefix": "neutron",
  "channel_id": "channel-public-neutron",
  "port_id": "transfer",
  "route_id": "neutron-public-wallet",
  "allowed_memo_prefixes": ["swap:", "stake:", "bridge:"],
  "allowed_action_types": ["swap", "stake", "bridge"],
  "assets": [
    {
      "asset_id": "eth",
      "source_denom": "ueth",
      "destination_denom": "ibc/ueth"
    }
  ],
  "notes": "Neutron testnet (pion-1) route. Set enabled=true and replace channel_id with the real channel after running transact link."
}
```

Save to `deploy/testnet/ibc/neutron-wallet-delivery.example.json`.

- [ ] **Step 2: Create `neutron-testnet.chain.example.json`**

```json
{
  "type": "cosmos",
  "value": {
    "chain-id": "pion-1",
    "rpc-addr": "https://rpc-falcron.pion-1.ntrn.tech:443",
    "grpc-addr": "https://grpc-falcron.pion-1.ntrn.tech:443",
    "account-prefix": "neutron",
    "keyring-backend": "test",
    "gas-adjustment": 1.3,
    "gas-prices": "1.3untrn",
    "debug": false,
    "timeout": "45s",
    "output-format": "json",
    "sign-mode": "direct"
  }
}
```

Save to `deploy/testnet/ibc/rly/neutron-testnet.chain.example.json`.

- [ ] **Step 3: Create `.env.public-ibc.neutron.local.example`**

```bash
# Neutron testnet (pion-1) destination overrides.
# Copy to .env.public-ibc.neutron.local and fill in your funded key mnemonic.
AEGISLINK_PUBLIC_BACKEND_NEUTRON_RPC_ADDR=https://rpc-falcron.pion-1.ntrn.tech:443
AEGISLINK_PUBLIC_BACKEND_NEUTRON_GRPC_ADDR=https://grpc-falcron.pion-1.ntrn.tech:443
AEGISLINK_PUBLIC_BACKEND_NEUTRON_LCD_BASE_URL=https://rest-falcron.pion-1.ntrn.tech
AEGISLINK_RLY_NEUTRON_KEY_NAME=neutron-demo
# Paste your funded Neutron testnet wallet mnemonic here (24 words):
AEGISLINK_RLY_NEUTRON_MNEMONIC=
```

Save to `.env.public-ibc.neutron.local.example`.

- [ ] **Step 4: Verify files exist**

```bash
ls deploy/testnet/ibc/neutron-wallet-delivery.example.json \
   deploy/testnet/ibc/rly/neutron-testnet.chain.example.json \
   .env.public-ibc.neutron.local.example
```
Expected: all three paths printed without error.

- [ ] **Step 5: Commit**

```bash
git add deploy/testnet/ibc/neutron-wallet-delivery.example.json \
        deploy/testnet/ibc/rly/neutron-testnet.chain.example.json \
        .env.public-ibc.neutron.local.example
git commit -m "feat(config): add Neutron testnet rly chain metadata, route manifest, and env example"
```

---

## Task 9: Update `start_public_bridge_backend.sh`

**Files:**
- Modify: `scripts/testnet/start_public_bridge_backend.sh`

- [ ] **Step 1: Add Neutron env var defaults (after the existing `RLY_TIMEOUT_SECONDS` block)**

After the line:
```bash
LINK_BLOCK_HISTORY="${AEGISLINK_PUBLIC_BACKEND_RLY_LINK_BLOCK_HISTORY:-5}"
```
Add:
```bash
NEUTRON_CHAIN_NAME="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_CHAIN_NAME:-pion-1}"
NEUTRON_CHAIN_ID="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_CHAIN_ID:-pion-1}"
NEUTRON_RPC_ADDR="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_RPC_ADDR:-https://rpc-falcron.pion-1.ntrn.tech:443}"
NEUTRON_GRPC_ADDR="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_GRPC_ADDR:-https://grpc-falcron.pion-1.ntrn.tech:443}"
NEUTRON_LCD_BASE_URL="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_LCD_BASE_URL:-https://rest-falcron.pion-1.ntrn.tech}"
NEUTRON_ACCOUNT_PREFIX="${AEGISLINK_RLY_NEUTRON_ACCOUNT_PREFIX:-neutron}"
NEUTRON_GAS_PRICE_DENOM="${AEGISLINK_RLY_NEUTRON_GAS_PRICE_DENOM:-untrn}"
NEUTRON_GAS_PRICE_AMOUNT="${AEGISLINK_RLY_NEUTRON_GAS_PRICE_AMOUNT:-1.3}"
NEUTRON_KEY_NAME="${AEGISLINK_RLY_NEUTRON_KEY_NAME:-neutron-demo}"
NEUTRON_MNEMONIC="${AEGISLINK_RLY_NEUTRON_MNEMONIC:-}"
NEUTRON_PATH_NAME="${AEGISLINK_PUBLIC_BACKEND_NEUTRON_PATH_NAME:-live-ntrn-ui-$RUN_ID}"
NEUTRON_ROUTE_ID="neutron-public-wallet"
```

- [ ] **Step 2: Update `write_rly_config` to accept a 4th arg and add the Neutron chain block**

Replace the `write_rly_config()` function signature line:
```bash
write_rly_config() {
  local rly_home="$1"
  local source_key_name="$2"
  local destination_key_name="$3"
```
with:
```bash
write_rly_config() {
  local rly_home="$1"
  local source_key_name="$2"
  local destination_key_name="$3"
  local neutron_key_name="$4"
```

Then inside the heredoc, after the closing of the `$DESTINATION_CHAIN_NAME:` block (after `sign-mode: direct`), add:

```yaml
  $NEUTRON_CHAIN_NAME:
    type: cosmos
    value:
      key: $neutron_key_name
      chain-id: $NEUTRON_CHAIN_ID
      rpc-addr: $NEUTRON_RPC_ADDR
      websocket-addr: ""
      grpc-addr: $NEUTRON_GRPC_ADDR
      account-prefix: $NEUTRON_ACCOUNT_PREFIX
      keyring-backend: test
      gas-adjustment: 1.3
      gas-prices: ${NEUTRON_GAS_PRICE_AMOUNT}${NEUTRON_GAS_PRICE_DENOM}
      debug: false
      timeout: ${RLY_TIMEOUT_SECONDS}s
      output-format: json
      sign-mode: direct
```

- [ ] **Step 3: Update `ensure_rly_home` to accept and ensure the Neutron key**

Replace:
```bash
ensure_rly_home() {
  local rly_home="$1"
  local source_key_name="$2"
  local destination_key_name="$3"
  local destination_mnemonic="${4:-}"

  write_rly_config "$rly_home" "$source_key_name" "$destination_key_name"
  ensure_relayer_key "aegislink-public-testnet-1" "$source_key_name" "$rly_home"
  ensure_relayer_key "$DESTINATION_CHAIN_NAME" "$destination_key_name" "$rly_home" "$destination_mnemonic"
}
```
with:
```bash
ensure_rly_home() {
  local rly_home="$1"
  local source_key_name="$2"
  local destination_key_name="$3"
  local destination_mnemonic="${4:-}"
  local neutron_key_name="$5"
  local neutron_mnemonic="${6:-}"

  write_rly_config "$rly_home" "$source_key_name" "$destination_key_name" "$neutron_key_name"
  ensure_relayer_key "aegislink-public-testnet-1" "$source_key_name" "$rly_home"
  ensure_relayer_key "$DESTINATION_CHAIN_NAME" "$destination_key_name" "$rly_home" "$destination_mnemonic"
  ensure_relayer_key "$NEUTRON_CHAIN_NAME" "$neutron_key_name" "$rly_home" "$neutron_mnemonic"
}
```

- [ ] **Step 4: Update `run_relayer_link_with_retry` to accept path name as `$1`**

Replace the function's opening and first `link_log` + `transact link` command lines. The function currently reads `$PATH_NAME` from the outer scope. Change it to read from `$1`:

```bash
run_relayer_link_with_retry() {
  local path_name="$1"
  local attempt=""
  local link_log=""
  local exit_code="0"
  local output=""
  local safe_path_name=""
  safe_path_name="$(printf '%s' "$path_name" | tr -cs '[:alnum:]' '_')"

  for ((attempt = 1; attempt <= LINK_RETRIES; attempt += 1)); do
    link_log="$RUNTIME_DIR/rly-link-${safe_path_name}-attempt-$attempt.log"
    echo "+ ./bin/relayer transact link $path_name --home $RLY_HOME --override --timeout $LINK_TIMEOUT_DURATION --max-retries $LINK_MAX_RETRIES --block-history $LINK_BLOCK_HISTORY --debug --log-level debug"

    set +e
    ./bin/relayer transact link "$path_name" \
      --home "$RLY_HOME" \
      --override \
      --timeout "$LINK_TIMEOUT_DURATION" \
      --max-retries "$LINK_MAX_RETRIES" \
      --block-history "$LINK_BLOCK_HISTORY" \
      --debug \
      --log-level debug >"$link_log" 2>&1
    exit_code="$?"
    set -e

    cat "$link_log"

    if [[ "$exit_code" == "0" ]]; then
      return 0
    fi

    output="$(cat "$link_log")"
    if (( attempt < LINK_RETRIES )) && public_bridge_link_error_is_retryable "$output"; then
      echo "transient relayer path-link failure on attempt $attempt/$LINK_RETRIES; retrying in ${LINK_RETRY_DELAY_SECONDS}s" >&2
      sleep "$LINK_RETRY_DELAY_SECONDS"
      continue
    fi

    return "$exit_code"
  done
}
```

- [ ] **Step 5: Add `neutron_key_has_funds` helper**

After the existing `destination_key_has_funds()` function, add:

```bash
neutron_key_has_funds() {
  local rly_home="$1"
  local neutron_key_name="$2"
  local balance_output=""

  balance_output="$(
    ./bin/relayer query balance "$NEUTRON_CHAIN_NAME" "$neutron_key_name" --home "$rly_home" -o json 2>/dev/null || true
  )"
  [[ "$balance_output" =~ \"amount\":\"[1-9][0-9]*\" ]] || \
    [[ "$balance_output" =~ \"balance\":\"[1-9][0-9]*[[:alpha:]]+\" ]]
}
```

- [ ] **Step 6: Update the `ensure_rly_home` call site to pass Neutron args**

Find the call (around line 316):
```bash
ensure_rly_home "$RLY_HOME" "$SOURCE_KEY_NAME" "$DESTINATION_KEY_NAME" "$DESTINATION_MNEMONIC"
```
Replace with:
```bash
ensure_rly_home "$RLY_HOME" "$SOURCE_KEY_NAME" "$DESTINATION_KEY_NAME" "$DESTINATION_MNEMONIC" "$NEUTRON_KEY_NAME" "$NEUTRON_MNEMONIC"
```

- [ ] **Step 7: Update the existing Osmosis `run_relayer_link_with_retry` call to pass path name**

Find:
```bash
run_relayer_link_with_retry
```
Replace with:
```bash
run_relayer_link_with_retry "$PATH_NAME"
```

- [ ] **Step 8: Add Neutron key funding check and path bootstrap after the Osmosis path setup**

After the Osmosis `set-route-profile` call (the `run go run ./chain/aegislink/cmd/aegislinkd tx set-route-profile ... --route-id osmosis-public-wallet` block), add:

```bash
NEUTRON_RELAYER_ADDRESS="$(
  ./bin/relayer keys show "$NEUTRON_CHAIN_NAME" "$NEUTRON_KEY_NAME" --home "$RLY_HOME" | tr -d '\r\n'
)"
if ! neutron_key_has_funds "$RLY_HOME" "$NEUTRON_KEY_NAME"; then
  echo "neutron relayer key is not funded yet" >&2
  echo "Relayer home: $RLY_HOME" >&2
  echo "Fund this address with testnet NTRN once, then rerun this same command:" >&2
  echo "  $NEUTRON_RELAYER_ADDRESS" >&2
  exit 1
fi

run ./bin/relayer paths new aegislink-public-testnet-1 "$NEUTRON_CHAIN_NAME" "$NEUTRON_PATH_NAME" --home "$RLY_HOME"
run_relayer_link_with_retry "$NEUTRON_PATH_NAME"

run go run ./chain/aegislink/cmd/aegislinkd tx set-route-profile \
  --home "$HOME_DIR" \
  --demo-node-ready-file "$READY_FILE" \
  --route-id neutron-public-wallet \
  --destination-chain-id pion-1 \
  --channel-id channel-1 \
  --asset-id eth \
  --destination-denom ibc/ueth \
  --enabled=true \
  --memo-prefixes "$AEGISLINK_PUBLIC_IBC_ALLOWED_MEMO_PREFIXES" \
  --action-types "$AEGISLINK_PUBLIC_IBC_ALLOWED_ACTION_TYPES"
```

- [ ] **Step 9: Export `AEGISLINK_RELAYER_RLY_PATH_MAP` before starting the relayer**

After the line:
```bash
export AEGISLINK_RELAYER_RLY_PATH_NAME="$PATH_NAME"
```
(or just before the `nohup go run ./relayer/cmd/public-bridge-relayer` line), add:

```bash
export AEGISLINK_RELAYER_RLY_PATH_MAP="osmosis-public-wallet:$PATH_NAME,neutron-public-wallet:$NEUTRON_PATH_NAME"
```

- [ ] **Step 10: Verify the script parses correctly**

```bash
bash -n scripts/testnet/start_public_bridge_backend.sh
```
Expected: no syntax errors (exits 0, no output).

- [ ] **Step 11: Run the full Go and frontend test suites one final time**

```bash
go test ./relayer/... ./chain/aegislink/... 2>&1 | tail -10
cd web && npm test 2>&1 | tail -10
```
Expected: all packages and test suites PASS.

- [ ] **Step 12: Final commit**

```bash
git add scripts/testnet/start_public_bridge_backend.sh
git commit -m "feat(scripts): add Neutron testnet (pion-1) to public bridge backend — two-destination rly bootstrap"
```
