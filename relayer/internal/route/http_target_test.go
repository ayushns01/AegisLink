package route

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestHTTPTargetSubmitsTransferAndParsesAck(t *testing.T) {
	t.Parallel()

	target := newHTTPTargetWithClient("http://mock-osmosis", &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/transfers" {
				t.Fatalf("expected /transfers, got %s", req.URL.Path)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			var envelope map[string]any
			if err := json.Unmarshal(payload, &envelope); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			transfer, ok := envelope["transfer"].(map[string]any)
			if !ok {
				t.Fatalf("expected transfer envelope, got %v", envelope)
			}
			packet, ok := envelope["packet"].(map[string]any)
			if !ok {
				t.Fatalf("expected packet envelope, got %v", envelope)
			}
			denomTrace, ok := envelope["denom_trace"].(map[string]any)
			if !ok {
				t.Fatalf("expected denom trace envelope, got %v", envelope)
			}
			action, ok := envelope["action"].(map[string]any)
			if !ok {
				t.Fatalf("expected action envelope, got %v", envelope)
			}
			if got := transfer["transfer_id"]; got != "ibc/eth.usdc/1" {
				t.Fatalf("expected transfer id ibc/eth.usdc/1, got %v", got)
			}
			if got := packet["sequence"]; got != float64(1) {
				t.Fatalf("expected packet sequence 1, got %v", got)
			}
			if got := packet["source_port"]; got != "transfer" {
				t.Fatalf("expected source port transfer, got %v", got)
			}
			if got := packet["source_channel"]; got != "channel-0" {
				t.Fatalf("expected source channel channel-0, got %v", got)
			}
			packetData, ok := packet["data"].(map[string]any)
			if !ok {
				t.Fatalf("expected packet data, got %v", packet)
			}
			if got := packetData["receiver"]; got != "osmo1recipient" {
				t.Fatalf("expected receiver osmo1recipient, got %v", got)
			}
			if got := packetData["memo"]; got != "swap:uosmo" {
				t.Fatalf("expected memo swap:uosmo, got %v", got)
			}
			if got := denomTrace["path"]; got != "transfer/channel-0" {
				t.Fatalf("expected denom trace path transfer/channel-0, got %v", got)
			}
			if got := denomTrace["base_denom"]; got != "eth.usdc" {
				t.Fatalf("expected base denom eth.usdc, got %v", got)
			}
			if got := denomTrace["ibc_denom"]; got != "ibc/uatom-usdc" {
				t.Fatalf("expected ibc denom ibc/uatom-usdc, got %v", got)
			}
			if got := action["type"]; got != "swap" {
				t.Fatalf("expected action type swap, got %v", got)
			}
			if got := action["target_denom"]; got != "uosmo" {
				t.Fatalf("expected target denom uosmo, got %v", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(newStaticReader(`{"status":"received"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	ack, err := target.SubmitTransfer(context.Background(), Transfer{
		TransferID:         "ibc/eth.usdc/1",
		AssetID:            "eth.usdc",
		Amount:             "25000000",
		Receiver:           "osmo1recipient",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		TimeoutHeight:      140,
		Memo:               "swap:uosmo",
	})
	if err != nil {
		t.Fatalf("submit transfer: %v", err)
	}
	if ack.Status != AckStatusReceived {
		t.Fatalf("expected received ack, got %q", ack.Status)
	}
}

func TestHTTPTargetCanFetchAndConfirmReadyAcks(t *testing.T) {
	t.Parallel()

	var confirmedPath string
	target := newHTTPTargetWithClient("http://mock-osmosis", &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.URL.Path == "/acks":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(newStaticReader(`[
						{"transfer_id":"ibc/eth.usdc/1","status":"completed"},
						{"transfer_id":"ibc/eth.usdc/2","status":"ack_failed","reason":"swap failed"}
					]`)),
					Header: make(http.Header),
				}, nil
			case req.Method == http.MethodPost && req.URL.Path == "/acks/confirm":
				confirmedPath = req.URL.Path + "?" + req.URL.RawQuery
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(newStaticReader(`{}`)),
					Header:     make(http.Header),
				}, nil
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
				return nil, nil
			}
		}),
	})

	acks, err := target.ReadyAcks(context.Background())
	if err != nil {
		t.Fatalf("ready acks: %v", err)
	}
	if len(acks) != 2 {
		t.Fatalf("expected 2 ready acks, got %d", len(acks))
	}
	if acks[0].TransferID != "ibc/eth.usdc/1" || acks[0].Status != AckStatusCompleted {
		t.Fatalf("unexpected first ack: %+v", acks[0])
	}
	if acks[1].TransferID != "ibc/eth.usdc/2" || acks[1].Status != AckStatusFailed || acks[1].Reason != "swap failed" {
		t.Fatalf("unexpected second ack: %+v", acks[1])
	}

	if err := target.ConfirmAck(context.Background(), "ibc/eth.usdc/1"); err != nil {
		t.Fatalf("confirm ack: %v", err)
	}
	if confirmedPath != "/acks/confirm?transfer_id=ibc%2Feth.usdc%2F1" {
		t.Fatalf("unexpected confirmed path %q", confirmedPath)
	}
}

func TestHTTPTargetReturnsTimeoutError(t *testing.T) {
	t.Parallel()

	target := newHTTPTargetWithClient("http://mock-osmosis", &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}),
	})
	_, err := target.SubmitTransfer(context.Background(), Transfer{
		TransferID:         "ibc/eth.usdc/1",
		AssetID:            "eth.usdc",
		Amount:             "25000000",
		Receiver:           "osmo1recipient",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		TimeoutHeight:      140,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if _, ok := err.(TimeoutError); !ok {
		t.Fatalf("expected TimeoutError, got %T", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type staticReader string

func newStaticReader(value string) *staticReader {
	reader := staticReader(value)
	return &reader
}

func (r *staticReader) Read(p []byte) (int, error) {
	if len(*r) == 0 {
		return 0, io.EOF
	}
	n := copy(p, *r)
	*r = (*r)[n:]
	return n, nil
}

func (r *staticReader) Close() error {
	return nil
}
