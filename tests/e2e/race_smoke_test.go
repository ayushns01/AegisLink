package e2e

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	governancekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/governance/keeper"
	ibcroutertypes "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/types"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
)

func TestRaceSmokeSerializedRuntimeAccess(t *testing.T) {
	t.Parallel()

	app, err := aegisapp.NewWithConfig(aegisapp.Config{
		AppName:               aegisapp.AppName,
		StatePath:             filepath.Join(t.TempDir(), "race-runtime.json"),
		AllowedSigners:        bridgetypes.DefaultHarnessSignerAddresses()[:3],
		RequiredThreshold:     2,
		GovernanceAuthorities: []string{"guardian-1"},
		Modules:               []string{"bridge", "bank", "registry", "limits", "pauser", "ibcrouter", "governance"},
	})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if err := app.RegisterAsset(sampleRuntimeAsset()); err != nil {
		t.Fatalf("register asset: %v", err)
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowSeconds: 600,
		MaxAmount:     mustBigAmount(t, "1000000000000000000"),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	if err := app.IBCRouterKeeper.SetRouteProfile(ibcroutertypes.RouteProfile{
		RouteID:            "osmosis-fast",
		DestinationChainID: "osmo-local-1",
		ChannelID:          "channel-0",
		Enabled:            true,
		Assets: []ibcroutertypes.AssetRoute{
			{AssetID: "eth.usdc", DestinationDenom: "ibc/uethusdc"},
		},
		Policy: ibcroutertypes.RoutePolicy{
			AllowedMemoPrefixes: []string{"swap:"},
			AllowedActionTypes:  []string{"swap"},
		},
	}); err != nil {
		t.Fatalf("set route profile: %v", err)
	}
	app.SetCurrentHeight(50)

	queryService := aegisapp.NewBridgeQueryService(app)
	routeQueryService := aegisapp.NewIBCRouterQueryService(app)
	govService := aegisapp.NewGovernanceTxService(app)

	var wg sync.WaitGroup
	errs := make(chan error, 32)

	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			claim := sampleAttestationDepositClaim(t, uint64(i+1))
			attestation := signedAttestationForClaim(t, claim, 0, 1)
			if _, err := app.SubmitDepositClaim(claim, attestation); err != nil {
				errs <- fmt.Errorf("submit claim %d: %w", i, err)
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			_ = app.Status()
			_, _ = queryService.GetClaim("")
			_ = routeQueryService.ListRoutes()
			_ = routeQueryService.ListTransfers()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			if err := govService.ApplyLimitUpdateProposal("guardian-1", governancekeeper.LimitUpdateProposal{
				ProposalID: fmt.Sprintf("limit-race-%d", i),
				Limit: limittypes.RateLimit{
					AssetID:       "eth.usdc",
					WindowSeconds: 600,
					MaxAmount:     mustBigAmount(t, "1000000000000000000"),
				},
			}); err != nil {
				errs <- fmt.Errorf("apply limit proposal %d: %w", i, err)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			if err := govService.ApplyRoutePolicyUpdateProposal("guardian-1", governancekeeper.RoutePolicyUpdateProposal{
				ProposalID: fmt.Sprintf("route-race-%d", i),
				RouteID:    "osmosis-fast",
				Policy: ibcroutertypes.RoutePolicy{
					AllowedMemoPrefixes: []string{"swap:"},
					AllowedActionTypes:  []string{"swap"},
				},
			}); err != nil {
				errs <- fmt.Errorf("apply route policy proposal %d: %w", i, err)
			}
		}
	}()

	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	status := app.Status()
	if status.ProcessedClaims != 6 {
		t.Fatalf("expected 6 processed claims after concurrent run, got %d", status.ProcessedClaims)
	}
	if status.GovernanceProposals != 6 {
		t.Fatalf("expected 6 governance proposals after concurrent run, got %d", status.GovernanceProposals)
	}
}
