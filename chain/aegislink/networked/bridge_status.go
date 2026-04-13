package networked

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
)

type BridgeSessionView struct {
	SourceTxHash      string `json:"sourceTxHash"`
	Status            string `json:"status"`
	MessageID         string `json:"messageId,omitempty"`
	TransferID        string `json:"transferId,omitempty"`
	DestinationTxHash string `json:"destinationTxHash,omitempty"`
	DestinationTxURL  string `json:"destinationTxUrl,omitempty"`
	ErrorMessage      string `json:"errorMessage,omitempty"`
}

type DestinationTxLookup struct {
	SourceChannelID string
	PacketSequence  uint64
}

type DestinationTxResult struct {
	TxHash string
	TxURL  string
}

type DestinationTxResolver interface {
	Resolve(context.Context, DestinationTxLookup) (DestinationTxResult, bool, error)
}

type LCDDestinationTxResolver struct {
	BaseURL string
	Client  *http.Client
}

func ResolveBridgeSessionView(
	ctx context.Context,
	app *aegisapp.App,
	sourceTxHash string,
	resolver DestinationTxResolver,
) (BridgeSessionView, error) {
	sourceTxHash = strings.TrimSpace(sourceTxHash)
	if sourceTxHash == "" {
		return BridgeSessionView{}, fmt.Errorf("missing source tx hash")
	}

	view := BridgeSessionView{
		SourceTxHash: sourceTxHash,
		Status:       "sepolia_confirming",
	}

	if app == nil {
		return view, nil
	}

	claim, ok := findClaimBySourceTxHash(app, sourceTxHash)
	if !ok {
		return view, nil
	}
	view.MessageID = claim.MessageID
	view.Status = "aegislink_processing"

	transfer, transport, ok := findTransferForClaim(app, claim)
	if !ok {
		return view, nil
	}
	view.TransferID = transfer.TransferID

	switch transfer.Status {
	case ibcrouterkeeper.TransferStatusPending:
		view.Status = "osmosis_pending"
	case ibcrouterkeeper.TransferStatusCompleted:
		view.Status = "completed"
		if resolver != nil && transport != nil {
			result, found, err := resolver.Resolve(ctx, DestinationTxLookup{
				SourceChannelID: transport.ChannelID,
				PacketSequence:  transport.PacketSequence,
			})
			if err != nil {
				view.ErrorMessage = err.Error()
			} else if found {
				view.DestinationTxHash = result.TxHash
				view.DestinationTxURL = result.TxURL
			}
		}
	case ibcrouterkeeper.TransferStatusAckFailed, ibcrouterkeeper.TransferStatusTimedOut, ibcrouterkeeper.TransferStatusRefunded:
		view.Status = "failed"
		view.ErrorMessage = strings.TrimSpace(transfer.FailureReason)
		if view.ErrorMessage == "" {
			view.ErrorMessage = string(transfer.Status)
		}
	default:
		view.Status = "osmosis_pending"
	}

	return view, nil
}

func (r LCDDestinationTxResolver) Resolve(ctx context.Context, lookup DestinationTxLookup) (DestinationTxResult, bool, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(r.BaseURL), "/")
	if baseURL == "" || strings.TrimSpace(lookup.SourceChannelID) == "" || lookup.PacketSequence == 0 {
		return DestinationTxResult{}, false, nil
	}

	queryURL, err := url.Parse(baseURL + "/cosmos/tx/v1beta1/txs")
	if err != nil {
		return DestinationTxResult{}, false, err
	}
	values := queryURL.Query()
	values.Add("events", fmt.Sprintf("recv_packet.packet_src_channel='%s'", lookup.SourceChannelID))
	values.Add("events", fmt.Sprintf("recv_packet.packet_sequence='%d'", lookup.PacketSequence))
	values.Set("pagination.limit", "1")
	queryURL.RawQuery = values.Encode()

	client := r.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return DestinationTxResult{}, false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return DestinationTxResult{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return DestinationTxResult{}, false, fmt.Errorf("destination tx lookup failed with %s", resp.Status)
	}

	var payload struct {
		TxResponses []struct {
			TxHash string `json:"txhash"`
		} `json:"tx_responses"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return DestinationTxResult{}, false, err
	}
	if len(payload.TxResponses) == 0 || strings.TrimSpace(payload.TxResponses[0].TxHash) == "" {
		return DestinationTxResult{}, false, nil
	}

	return DestinationTxResult{
		TxHash: payload.TxResponses[0].TxHash,
	}, true, nil
}

func findClaimBySourceTxHash(app *aegisapp.App, sourceTxHash string) (bridgekeeper.ClaimRecordSnapshot, bool) {
	for _, claim := range app.BridgeKeeper.ExportState().ProcessedClaims {
		if strings.EqualFold(strings.TrimSpace(claim.SourceTxHash), strings.TrimSpace(sourceTxHash)) {
			return claim, true
		}
	}
	return bridgekeeper.ClaimRecordSnapshot{}, false
}

func findTransferForClaim(
	app *aegisapp.App,
	claim bridgekeeper.ClaimRecordSnapshot,
) (ibcrouterkeeper.TransferRecordSnapshot, *ibcrouterkeeper.TransportRecordSnapshot, bool) {
	routerState := app.IBCRouterKeeper.ExportState()
	matches := make([]ibcrouterkeeper.TransferRecordSnapshot, 0, len(routerState.Transfers))
	for _, transfer := range routerState.Transfers {
		if strings.TrimSpace(transfer.AssetID) != strings.TrimSpace(claim.AssetID) {
			continue
		}
		if strings.TrimSpace(transfer.Receiver) != strings.TrimSpace(claim.Recipient) {
			continue
		}
		if strings.TrimSpace(transfer.Amount) != strings.TrimSpace(claim.Amount) {
			continue
		}
		matches = append(matches, transfer)
	}
	if len(matches) == 0 {
		return ibcrouterkeeper.TransferRecordSnapshot{}, nil, false
	}

	slices.SortFunc(matches, func(a, b ibcrouterkeeper.TransferRecordSnapshot) int {
		if weight := transferStatusWeight(a.Status) - transferStatusWeight(b.Status); weight != 0 {
			return -weight
		}
		return strings.Compare(b.TransferID, a.TransferID)
	})

	selected := matches[0]
	for _, transport := range routerState.Transport {
		if transport.TransferID == selected.TransferID {
			copy := transport
			return selected, &copy, true
		}
	}
	return selected, nil, true
}

func transferStatusWeight(status ibcrouterkeeper.TransferStatus) int {
	switch status {
	case ibcrouterkeeper.TransferStatusCompleted:
		return 4
	case ibcrouterkeeper.TransferStatusPending:
		return 3
	case ibcrouterkeeper.TransferStatusAckFailed:
		return 2
	case ibcrouterkeeper.TransferStatusTimedOut:
		return 1
	case ibcrouterkeeper.TransferStatusRefunded:
		return 0
	default:
		return -1
	}
}
