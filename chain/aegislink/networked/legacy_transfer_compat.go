package networked

import (
	"fmt"
	"strings"

	querytypes "github.com/cosmos/cosmos-sdk/types/query"
	proto "github.com/cosmos/gogoproto/proto"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
)

type legacyTransferDenomTrace struct {
	Path      string `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
	BaseDenom string `protobuf:"bytes,2,opt,name=base_denom,json=baseDenom,proto3" json:"base_denom,omitempty"`
}

func (m *legacyTransferDenomTrace) Reset()         { *m = legacyTransferDenomTrace{} }
func (m *legacyTransferDenomTrace) String() string { return proto.CompactTextString(m) }
func (*legacyTransferDenomTrace) ProtoMessage()    {}

type legacyQueryDenomTraceRequest struct {
	Hash string `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
}

func (m *legacyQueryDenomTraceRequest) Reset()         { *m = legacyQueryDenomTraceRequest{} }
func (m *legacyQueryDenomTraceRequest) String() string { return proto.CompactTextString(m) }
func (*legacyQueryDenomTraceRequest) ProtoMessage()    {}

type legacyQueryDenomTraceResponse struct {
	DenomTrace *legacyTransferDenomTrace `protobuf:"bytes,1,opt,name=denom_trace,json=denomTrace,proto3" json:"denom_trace,omitempty"`
}

func (m *legacyQueryDenomTraceResponse) Reset()         { *m = legacyQueryDenomTraceResponse{} }
func (m *legacyQueryDenomTraceResponse) String() string { return proto.CompactTextString(m) }
func (*legacyQueryDenomTraceResponse) ProtoMessage()    {}

type legacyQueryDenomTracesRequest struct {
	Pagination *querytypes.PageRequest `protobuf:"bytes,1,opt,name=pagination,proto3" json:"pagination,omitempty"`
}

func (m *legacyQueryDenomTracesRequest) Reset()         { *m = legacyQueryDenomTracesRequest{} }
func (m *legacyQueryDenomTracesRequest) String() string { return proto.CompactTextString(m) }
func (*legacyQueryDenomTracesRequest) ProtoMessage()    {}

type legacyQueryDenomTracesResponse struct {
	DenomTraces []*legacyTransferDenomTrace `protobuf:"bytes,1,rep,name=denom_traces,json=denomTraces,proto3" json:"denom_traces,omitempty"`
	Pagination  *querytypes.PageResponse    `protobuf:"bytes,2,opt,name=pagination,proto3" json:"pagination,omitempty"`
}

func (m *legacyQueryDenomTracesResponse) Reset()         { *m = legacyQueryDenomTracesResponse{} }
func (m *legacyQueryDenomTracesResponse) String() string { return proto.CompactTextString(m) }
func (*legacyQueryDenomTracesResponse) ProtoMessage()    {}

func legacyTransferTraceFromDenom(denom transfertypes.Denom) *legacyTransferDenomTrace {
	if denom.IsNative() {
		return &legacyTransferDenomTrace{BaseDenom: denom.Base}
	}
	traceParts := make([]string, 0, len(denom.Trace)*2)
	for _, hop := range denom.Trace {
		traceParts = append(traceParts, hop.PortId, hop.ChannelId)
	}
	return &legacyTransferDenomTrace{
		Path:      strings.Join(traceParts, "/"),
		BaseDenom: denom.Base,
	}
}

func (m *legacyTransferDenomTrace) GetFullDenomPath() string {
	if m == nil || m.Path == "" {
		if m == nil {
			return ""
		}
		return m.BaseDenom
	}
	return fmt.Sprintf("%s/%s", m.Path, m.BaseDenom)
}
