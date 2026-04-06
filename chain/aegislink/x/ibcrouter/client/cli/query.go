package cli

import (
	ibcrouterv1 "github.com/ayushns01/aegislink/chain/aegislink/gen/go/aegislink/ibcrouter/v1"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
)

func RoutesResponse(routes []ibcrouterkeeper.Route) *ibcrouterv1.RoutesResponse {
	response := &ibcrouterv1.RoutesResponse{
		Routes: make([]*ibcrouterv1.Route, 0, len(routes)),
	}
	for _, route := range routes {
		response.Routes = append(response.Routes, &ibcrouterv1.Route{
			AssetId:            route.AssetID,
			DestinationChainId: route.DestinationChainID,
			ChannelId:          route.ChannelID,
			DestinationDenom:   route.DestinationDenom,
			Enabled:            route.Enabled,
		})
	}
	return response
}

func TransfersResponse(transfers []ibcrouterkeeper.TransferRecord) *ibcrouterv1.TransfersResponse {
	response := &ibcrouterv1.TransfersResponse{
		Transfers: make([]*ibcrouterv1.Transfer, 0, len(transfers)),
	}
	for _, transfer := range transfers {
		response.Transfers = append(response.Transfers, &ibcrouterv1.Transfer{
			TransferId:         transfer.TransferID,
			AssetId:            transfer.AssetID,
			Amount:             transfer.Amount.String(),
			Receiver:           transfer.Receiver,
			DestinationChainId: transfer.DestinationChainID,
			ChannelId:          transfer.ChannelID,
			DestinationDenom:   transfer.DestinationDenom,
			TimeoutHeight:      transfer.TimeoutHeight,
			Memo:               transfer.Memo,
			Status:             string(transfer.Status),
			FailureReason:      transfer.FailureReason,
		})
	}
	return response
}
