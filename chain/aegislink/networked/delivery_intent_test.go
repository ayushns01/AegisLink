package networked

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
)

func TestDemoNodeServeHTTPRegistersAndListsDeliveryIntents(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "delivery-intents-home")
	node := DemoNode{
		appConfig: aegisapp.Config{HomeDir: homeDir},
	}
	ready := ReadyState{Status: "ready", ChainID: "aegislink-public-testnet-1"}

	body, err := json.Marshal(DeliveryIntent{
		SourceTxHash: "0x422d075a86656b27694780b3ad553abee1dded6f3fb5bfa805137a3da64f30b8",
		Sender:       "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4",
		RouteID:      "osmosis-public-wallet",
		AssetID:      "eth",
		Amount:       "1000000000000000",
		Receiver:     "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
	})
	if err != nil {
		t.Fatalf("marshal delivery intent: %v", err)
	}

	postReq := httptest.NewRequest(http.MethodPost, "/delivery-intents", bytes.NewReader(body))
	postReq.Header.Set("Content-Type", "application/json")
	postRecorder := httptest.NewRecorder()

	node.serveHTTP(postRecorder, postReq, ready)

	if postRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", postRecorder.Code, postRecorder.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/delivery-intents", nil)
	getRecorder := httptest.NewRecorder()

	node.serveHTTP(getRecorder, getReq, ready)

	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", getRecorder.Code, getRecorder.Body.String())
	}

	var intents []DeliveryIntent
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &intents); err != nil {
		t.Fatalf("decode intents response: %v", err)
	}
	if len(intents) != 1 {
		t.Fatalf("expected one stored intent, got %+v", intents)
	}
	if intents[0].Receiver != "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8" {
		t.Fatalf("unexpected stored intent: %+v", intents[0])
	}
}
