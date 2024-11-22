// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.27.1
// source: gov_params.proto

package types

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GovParamsProto struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Version                 int64  `protobuf:"varint,1,opt,name=version,proto3" json:"version,omitempty"`
	MaxValidatorCnt         int64  `protobuf:"varint,2,opt,name=max_validator_cnt,json=maxValidatorCnt,proto3" json:"max_validator_cnt,omitempty"`
	XGasPrice               []byte `protobuf:"bytes,3,opt,name=_gas_price,json=GasPrice,proto3" json:"_gas_price,omitempty"`
	XRewardPerPower         []byte `protobuf:"bytes,4,opt,name=_reward_per_power,json=RewardPerPower,proto3" json:"_reward_per_power,omitempty"`
	LazyUnstakingBlocks     int64  `protobuf:"varint,5,opt,name=lazy_unstaking_blocks,json=lazyUnstakingBlocks,proto3" json:"lazy_unstaking_blocks,omitempty"`
	LazyApplyingBlocks      int64  `protobuf:"varint,6,opt,name=lazy_applying_blocks,json=lazyApplyingBlocks,proto3" json:"lazy_applying_blocks,omitempty"`
	MinTrxGas               uint64 `protobuf:"varint,7,opt,name=min_trx_gas,json=minTrxGas,proto3" json:"min_trx_gas,omitempty"`
	MaxTrxGas               uint64 `protobuf:"varint,8,opt,name=max_trx_gas,json=maxTrxGas,proto3" json:"max_trx_gas,omitempty"`
	MaxBlockGas             uint64 `protobuf:"varint,9,opt,name=max_block_gas,json=maxBlockGas,proto3" json:"max_block_gas,omitempty"`
	MinVotingPeriodBlocks   int64  `protobuf:"varint,10,opt,name=min_voting_period_blocks,json=minVotingPeriodBlocks,proto3" json:"min_voting_period_blocks,omitempty"`
	MaxVotingPeriodBlocks   int64  `protobuf:"varint,11,opt,name=max_voting_period_blocks,json=maxVotingPeriodBlocks,proto3" json:"max_voting_period_blocks,omitempty"`
	MinSelfStakeRatio       int64  `protobuf:"varint,12,opt,name=min_self_stake_ratio,json=minSelfStakeRatio,proto3" json:"min_self_stake_ratio,omitempty"`
	MaxUpdatableStakeRatio  int64  `protobuf:"varint,13,opt,name=max_updatable_stake_ratio,json=maxUpdatableStakeRatio,proto3" json:"max_updatable_stake_ratio,omitempty"`
	MaxIndividualStakeRatio int64  `protobuf:"varint,14,opt,name=max_individual_stake_ratio,json=maxIndividualStakeRatio,proto3" json:"max_individual_stake_ratio,omitempty"`
	SlashRatio              int64  `protobuf:"varint,15,opt,name=slash_ratio,json=slashRatio,proto3" json:"slash_ratio,omitempty"`
	XMinValidatorStake      []byte `protobuf:"bytes,16,opt,name=_min_validator_stake,json=MinValidatorStake,proto3" json:"_min_validator_stake,omitempty"`
	XMinDelegatorStake      []byte `protobuf:"bytes,19,opt,name=_min_delegator_stake,json=MinDelegatorStake,proto3" json:"_min_delegator_stake,omitempty"`
	SignedBlocksWindow      int64  `protobuf:"varint,17,opt,name=signed_blocks_window,json=signedBlocksWindow,proto3" json:"signed_blocks_window,omitempty"`
	MinSignedBlocks         int64  `protobuf:"varint,18,opt,name=min_signed_blocks,json=minSignedBlocks,proto3" json:"min_signed_blocks,omitempty"`
}

func (x *GovParamsProto) Reset() {
	*x = GovParamsProto{}
	if protoimpl.UnsafeEnabled {
		mi := &file_gov_params_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GovParamsProto) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GovParamsProto) ProtoMessage() {}

func (x *GovParamsProto) ProtoReflect() protoreflect.Message {
	mi := &file_gov_params_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GovParamsProto.ProtoReflect.Descriptor instead.
func (*GovParamsProto) Descriptor() ([]byte, []int) {
	return file_gov_params_proto_rawDescGZIP(), []int{0}
}

func (x *GovParamsProto) GetVersion() int64 {
	if x != nil {
		return x.Version
	}
	return 0
}

func (x *GovParamsProto) GetMaxValidatorCnt() int64 {
	if x != nil {
		return x.MaxValidatorCnt
	}
	return 0
}

func (x *GovParamsProto) GetXGasPrice() []byte {
	if x != nil {
		return x.XGasPrice
	}
	return nil
}

func (x *GovParamsProto) GetXRewardPerPower() []byte {
	if x != nil {
		return x.XRewardPerPower
	}
	return nil
}

func (x *GovParamsProto) GetLazyUnstakingBlocks() int64 {
	if x != nil {
		return x.LazyUnstakingBlocks
	}
	return 0
}

func (x *GovParamsProto) GetLazyApplyingBlocks() int64 {
	if x != nil {
		return x.LazyApplyingBlocks
	}
	return 0
}

func (x *GovParamsProto) GetMinTrxGas() uint64 {
	if x != nil {
		return x.MinTrxGas
	}
	return 0
}

func (x *GovParamsProto) GetMaxTrxGas() uint64 {
	if x != nil {
		return x.MaxTrxGas
	}
	return 0
}

func (x *GovParamsProto) GetMaxBlockGas() uint64 {
	if x != nil {
		return x.MaxBlockGas
	}
	return 0
}

func (x *GovParamsProto) GetMinVotingPeriodBlocks() int64 {
	if x != nil {
		return x.MinVotingPeriodBlocks
	}
	return 0
}

func (x *GovParamsProto) GetMaxVotingPeriodBlocks() int64 {
	if x != nil {
		return x.MaxVotingPeriodBlocks
	}
	return 0
}

func (x *GovParamsProto) GetMinSelfStakeRatio() int64 {
	if x != nil {
		return x.MinSelfStakeRatio
	}
	return 0
}

func (x *GovParamsProto) GetMaxUpdatableStakeRatio() int64 {
	if x != nil {
		return x.MaxUpdatableStakeRatio
	}
	return 0
}

func (x *GovParamsProto) GetMaxIndividualStakeRatio() int64 {
	if x != nil {
		return x.MaxIndividualStakeRatio
	}
	return 0
}

func (x *GovParamsProto) GetSlashRatio() int64 {
	if x != nil {
		return x.SlashRatio
	}
	return 0
}

func (x *GovParamsProto) GetXMinValidatorStake() []byte {
	if x != nil {
		return x.XMinValidatorStake
	}
	return nil
}

func (x *GovParamsProto) GetXMinDelegatorStake() []byte {
	if x != nil {
		return x.XMinDelegatorStake
	}
	return nil
}

func (x *GovParamsProto) GetSignedBlocksWindow() int64 {
	if x != nil {
		return x.SignedBlocksWindow
	}
	return 0
}

func (x *GovParamsProto) GetMinSignedBlocks() int64 {
	if x != nil {
		return x.MinSignedBlocks
	}
	return 0
}

var File_gov_params_proto protoreflect.FileDescriptor

var file_gov_params_proto_rawDesc = []byte{
	0x0a, 0x10, 0x67, 0x6f, 0x76, 0x5f, 0x70, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x05, 0x74, 0x79, 0x70, 0x65, 0x73, 0x22, 0xe5, 0x06, 0x0a, 0x0e, 0x47, 0x6f,
	0x76, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x18, 0x0a, 0x07,
	0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x76,
	0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x2a, 0x0a, 0x11, 0x6d, 0x61, 0x78, 0x5f, 0x76, 0x61,
	0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x5f, 0x63, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x03, 0x52, 0x0f, 0x6d, 0x61, 0x78, 0x56, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x43,
	0x6e, 0x74, 0x12, 0x1c, 0x0a, 0x0a, 0x5f, 0x67, 0x61, 0x73, 0x5f, 0x70, 0x72, 0x69, 0x63, 0x65,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x08, 0x47, 0x61, 0x73, 0x50, 0x72, 0x69, 0x63, 0x65,
	0x12, 0x29, 0x0a, 0x11, 0x5f, 0x72, 0x65, 0x77, 0x61, 0x72, 0x64, 0x5f, 0x70, 0x65, 0x72, 0x5f,
	0x70, 0x6f, 0x77, 0x65, 0x72, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0e, 0x52, 0x65, 0x77,
	0x61, 0x72, 0x64, 0x50, 0x65, 0x72, 0x50, 0x6f, 0x77, 0x65, 0x72, 0x12, 0x32, 0x0a, 0x15, 0x6c,
	0x61, 0x7a, 0x79, 0x5f, 0x75, 0x6e, 0x73, 0x74, 0x61, 0x6b, 0x69, 0x6e, 0x67, 0x5f, 0x62, 0x6c,
	0x6f, 0x63, 0x6b, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x52, 0x13, 0x6c, 0x61, 0x7a, 0x79,
	0x55, 0x6e, 0x73, 0x74, 0x61, 0x6b, 0x69, 0x6e, 0x67, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x12,
	0x30, 0x0a, 0x14, 0x6c, 0x61, 0x7a, 0x79, 0x5f, 0x61, 0x70, 0x70, 0x6c, 0x79, 0x69, 0x6e, 0x67,
	0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28, 0x03, 0x52, 0x12, 0x6c,
	0x61, 0x7a, 0x79, 0x41, 0x70, 0x70, 0x6c, 0x79, 0x69, 0x6e, 0x67, 0x42, 0x6c, 0x6f, 0x63, 0x6b,
	0x73, 0x12, 0x1e, 0x0a, 0x0b, 0x6d, 0x69, 0x6e, 0x5f, 0x74, 0x72, 0x78, 0x5f, 0x67, 0x61, 0x73,
	0x18, 0x07, 0x20, 0x01, 0x28, 0x04, 0x52, 0x09, 0x6d, 0x69, 0x6e, 0x54, 0x72, 0x78, 0x47, 0x61,
	0x73, 0x12, 0x1e, 0x0a, 0x0b, 0x6d, 0x61, 0x78, 0x5f, 0x74, 0x72, 0x78, 0x5f, 0x67, 0x61, 0x73,
	0x18, 0x08, 0x20, 0x01, 0x28, 0x04, 0x52, 0x09, 0x6d, 0x61, 0x78, 0x54, 0x72, 0x78, 0x47, 0x61,
	0x73, 0x12, 0x22, 0x0a, 0x0d, 0x6d, 0x61, 0x78, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x67,
	0x61, 0x73, 0x18, 0x09, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0b, 0x6d, 0x61, 0x78, 0x42, 0x6c, 0x6f,
	0x63, 0x6b, 0x47, 0x61, 0x73, 0x12, 0x37, 0x0a, 0x18, 0x6d, 0x69, 0x6e, 0x5f, 0x76, 0x6f, 0x74,
	0x69, 0x6e, 0x67, 0x5f, 0x70, 0x65, 0x72, 0x69, 0x6f, 0x64, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b,
	0x73, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x03, 0x52, 0x15, 0x6d, 0x69, 0x6e, 0x56, 0x6f, 0x74, 0x69,
	0x6e, 0x67, 0x50, 0x65, 0x72, 0x69, 0x6f, 0x64, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x12, 0x37,
	0x0a, 0x18, 0x6d, 0x61, 0x78, 0x5f, 0x76, 0x6f, 0x74, 0x69, 0x6e, 0x67, 0x5f, 0x70, 0x65, 0x72,
	0x69, 0x6f, 0x64, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x15, 0x6d, 0x61, 0x78, 0x56, 0x6f, 0x74, 0x69, 0x6e, 0x67, 0x50, 0x65, 0x72, 0x69, 0x6f,
	0x64, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x12, 0x2f, 0x0a, 0x14, 0x6d, 0x69, 0x6e, 0x5f, 0x73,
	0x65, 0x6c, 0x66, 0x5f, 0x73, 0x74, 0x61, 0x6b, 0x65, 0x5f, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x18,
	0x0c, 0x20, 0x01, 0x28, 0x03, 0x52, 0x11, 0x6d, 0x69, 0x6e, 0x53, 0x65, 0x6c, 0x66, 0x53, 0x74,
	0x61, 0x6b, 0x65, 0x52, 0x61, 0x74, 0x69, 0x6f, 0x12, 0x39, 0x0a, 0x19, 0x6d, 0x61, 0x78, 0x5f,
	0x75, 0x70, 0x64, 0x61, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x5f, 0x73, 0x74, 0x61, 0x6b, 0x65, 0x5f,
	0x72, 0x61, 0x74, 0x69, 0x6f, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x03, 0x52, 0x16, 0x6d, 0x61, 0x78,
	0x55, 0x70, 0x64, 0x61, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x53, 0x74, 0x61, 0x6b, 0x65, 0x52, 0x61,
	0x74, 0x69, 0x6f, 0x12, 0x3b, 0x0a, 0x1a, 0x6d, 0x61, 0x78, 0x5f, 0x69, 0x6e, 0x64, 0x69, 0x76,
	0x69, 0x64, 0x75, 0x61, 0x6c, 0x5f, 0x73, 0x74, 0x61, 0x6b, 0x65, 0x5f, 0x72, 0x61, 0x74, 0x69,
	0x6f, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x03, 0x52, 0x17, 0x6d, 0x61, 0x78, 0x49, 0x6e, 0x64, 0x69,
	0x76, 0x69, 0x64, 0x75, 0x61, 0x6c, 0x53, 0x74, 0x61, 0x6b, 0x65, 0x52, 0x61, 0x74, 0x69, 0x6f,
	0x12, 0x1f, 0x0a, 0x0b, 0x73, 0x6c, 0x61, 0x73, 0x68, 0x5f, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x18,
	0x0f, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0a, 0x73, 0x6c, 0x61, 0x73, 0x68, 0x52, 0x61, 0x74, 0x69,
	0x6f, 0x12, 0x2f, 0x0a, 0x14, 0x5f, 0x6d, 0x69, 0x6e, 0x5f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61,
	0x74, 0x6f, 0x72, 0x5f, 0x73, 0x74, 0x61, 0x6b, 0x65, 0x18, 0x10, 0x20, 0x01, 0x28, 0x0c, 0x52,
	0x11, 0x4d, 0x69, 0x6e, 0x56, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x53, 0x74, 0x61,
	0x6b, 0x65, 0x12, 0x2f, 0x0a, 0x14, 0x5f, 0x6d, 0x69, 0x6e, 0x5f, 0x64, 0x65, 0x6c, 0x65, 0x67,
	0x61, 0x74, 0x6f, 0x72, 0x5f, 0x73, 0x74, 0x61, 0x6b, 0x65, 0x18, 0x13, 0x20, 0x01, 0x28, 0x0c,
	0x52, 0x11, 0x4d, 0x69, 0x6e, 0x44, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x6f, 0x72, 0x53, 0x74,
	0x61, 0x6b, 0x65, 0x12, 0x30, 0x0a, 0x14, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x5f, 0x62, 0x6c,
	0x6f, 0x63, 0x6b, 0x73, 0x5f, 0x77, 0x69, 0x6e, 0x64, 0x6f, 0x77, 0x18, 0x11, 0x20, 0x01, 0x28,
	0x03, 0x52, 0x12, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x57,
	0x69, 0x6e, 0x64, 0x6f, 0x77, 0x12, 0x2a, 0x0a, 0x11, 0x6d, 0x69, 0x6e, 0x5f, 0x73, 0x69, 0x67,
	0x6e, 0x65, 0x64, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x18, 0x12, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x0f, 0x6d, 0x69, 0x6e, 0x53, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x42, 0x6c, 0x6f, 0x63, 0x6b,
	0x73, 0x42, 0x2b, 0x5a, 0x29, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x62, 0x65, 0x61, 0x74, 0x6f, 0x7a, 0x2f, 0x62, 0x65, 0x61, 0x74, 0x6f, 0x7a, 0x2d, 0x67, 0x6f,
	0x2f, 0x63, 0x74, 0x72, 0x6c, 0x65, 0x72, 0x73, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_gov_params_proto_rawDescOnce sync.Once
	file_gov_params_proto_rawDescData = file_gov_params_proto_rawDesc
)

func file_gov_params_proto_rawDescGZIP() []byte {
	file_gov_params_proto_rawDescOnce.Do(func() {
		file_gov_params_proto_rawDescData = protoimpl.X.CompressGZIP(file_gov_params_proto_rawDescData)
	})
	return file_gov_params_proto_rawDescData
}

var file_gov_params_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_gov_params_proto_goTypes = []any{
	(*GovParamsProto)(nil), // 0: types.GovParamsProto
}
var file_gov_params_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_gov_params_proto_init() }
func file_gov_params_proto_init() {
	if File_gov_params_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_gov_params_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*GovParamsProto); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_gov_params_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_gov_params_proto_goTypes,
		DependencyIndexes: file_gov_params_proto_depIdxs,
		MessageInfos:      file_gov_params_proto_msgTypes,
	}.Build()
	File_gov_params_proto = out.File
	file_gov_params_proto_rawDesc = nil
	file_gov_params_proto_goTypes = nil
	file_gov_params_proto_depIdxs = nil
}
