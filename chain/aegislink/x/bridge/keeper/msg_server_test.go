package keeper

import "testing"

func TestMsgServerSubmitDepositClaimDelegatesToKeeper(t *testing.T) {
	keeper, claim, attestation, _, _, _ := newKeeperFixture(t)
	server := NewMsgServer(keeper)

	result, err := server.SubmitDepositClaim(claim, attestation)
	if err != nil {
		t.Fatalf("expected msg server to accept valid claim, got %v", err)
	}
	if result.Status != ClaimStatusAccepted {
		t.Fatalf("expected accepted status, got %q", result.Status)
	}
}
