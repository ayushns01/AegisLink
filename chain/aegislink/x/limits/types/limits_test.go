package types

import (
	"errors"
	"math/big"
	"testing"
)

func TestRateLimitValidateBasic(t *testing.T) {
	limit := RateLimit{
		AssetID:       "eth.usdc",
		WindowBlocks: 600,
		MaxAmount:     mustAmount("1000000000000000000000000000000"),
	}

	if err := limit.ValidateBasic(); err != nil {
		t.Fatalf("expected valid rate limit, got error: %v", err)
	}

	cases := map[string]func(*RateLimit){
		"missing asset id":    func(l *RateLimit) { l.AssetID = "" },
		"missing window":      func(l *RateLimit) { l.WindowBlocks = 0 },
		"missing max amount":  func(l *RateLimit) { l.MaxAmount = nil },
		"negative max amount": func(l *RateLimit) { l.MaxAmount = big.NewInt(-1) },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			limit := limit
			mutate(&limit)
			if err := limit.ValidateBasic(); !errors.Is(err, ErrInvalidRateLimit) {
				t.Fatalf("expected invalid rate limit error, got: %v", err)
			}
		})
	}
}

func mustAmount(value string) *big.Int {
	amount, ok := new(big.Int).SetString(value, 10)
	if !ok {
		panic("invalid test amount")
	}
	return amount
}
