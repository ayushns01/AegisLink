package app

import "sort"

type TxConfig struct {
	SignMode      string `json:"sign_mode"`
	BroadcastMode string `json:"broadcast_mode"`
}

type EncodingConfig struct {
	CodecName         string   `json:"codec_name"`
	InterfaceRegistry []string `json:"interface_registry"`
	TxConfig          TxConfig `json:"tx_config"`
}

func DefaultEncodingConfig(modules []string) EncodingConfig {
	registry := make([]string, 0, len(modules)+2)
	for _, moduleName := range modules {
		registry = append(registry, moduleName+".msg")
	}
	registry = append(registry, "sdk.Msg", "sdk.Tx")
	sort.Strings(registry)

	return EncodingConfig{
		CodecName:         "proto-json",
		InterfaceRegistry: registry,
		TxConfig: TxConfig{
			SignMode:      "direct",
			BroadcastMode: "sync",
		},
	}
}
