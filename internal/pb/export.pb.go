// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.21.0
// 	protoc        v3.11.4
// source: internal/pb/export.proto

package pb

import (
	reflect "reflect"
	sync "sync"

	proto "github.com/golang/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type ExposureKeyExport struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Time window of keys in this file based on arrival to server, in UTC
	StartTimestamp *uint64 `protobuf:"fixed64,1,opt,name=startTimestamp" json:"startTimestamp,omitempty"`
	EndTimestamp   *uint64 `protobuf:"fixed64,2,opt,name=endTimestamp" json:"endTimestamp,omitempty"`
	// Region for which these keys came from (e.g., country)
	Region *string `protobuf:"bytes,3,opt,name=region" json:"region,omitempty"`
	// E.g., Batch 2 of 10
	BatchNum  *int32 `protobuf:"varint,4,opt,name=batchNum" json:"batchNum,omitempty"`
	BatchSize *int32 `protobuf:"varint,5,opt,name=batchSize" json:"batchSize,omitempty"`
	// Packed bytes of repeated exposure keys
	ExposureKeys []byte `protobuf:"bytes,6,opt,name=exposureKeys" json:"exposureKeys,omitempty"` // number of keys = length / 16 bytes per key
	// Array of single byte ints of transmission risks, with indexes corresponding to keys
	TransmissionRisks []byte `protobuf:"bytes,7,opt,name=transmissionRisks" json:"transmissionRisks,omitempty"`
	// Arrays of two byte ints (little endian) for interval and rolling period
	IntervalNumbers []byte `protobuf:"bytes,8,opt,name=intervalNumbers" json:"intervalNumbers,omitempty"`
	RollingPeriods  []byte `protobuf:"bytes,9,opt,name=rollingPeriods" json:"rollingPeriods,omitempty"`
}

func (x *ExposureKeyExport) Reset() {
	*x = ExposureKeyExport{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_pb_export_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ExposureKeyExport) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExposureKeyExport) ProtoMessage() {}

func (x *ExposureKeyExport) ProtoReflect() protoreflect.Message {
	mi := &file_internal_pb_export_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExposureKeyExport.ProtoReflect.Descriptor instead.
func (*ExposureKeyExport) Descriptor() ([]byte, []int) {
	return file_internal_pb_export_proto_rawDescGZIP(), []int{0}
}

func (x *ExposureKeyExport) GetStartTimestamp() uint64 {
	if x != nil && x.StartTimestamp != nil {
		return *x.StartTimestamp
	}
	return 0
}

func (x *ExposureKeyExport) GetEndTimestamp() uint64 {
	if x != nil && x.EndTimestamp != nil {
		return *x.EndTimestamp
	}
	return 0
}

func (x *ExposureKeyExport) GetRegion() string {
	if x != nil && x.Region != nil {
		return *x.Region
	}
	return ""
}

func (x *ExposureKeyExport) GetBatchNum() int32 {
	if x != nil && x.BatchNum != nil {
		return *x.BatchNum
	}
	return 0
}

func (x *ExposureKeyExport) GetBatchSize() int32 {
	if x != nil && x.BatchSize != nil {
		return *x.BatchSize
	}
	return 0
}

func (x *ExposureKeyExport) GetExposureKeys() []byte {
	if x != nil {
		return x.ExposureKeys
	}
	return nil
}

func (x *ExposureKeyExport) GetTransmissionRisks() []byte {
	if x != nil {
		return x.TransmissionRisks
	}
	return nil
}

func (x *ExposureKeyExport) GetIntervalNumbers() []byte {
	if x != nil {
		return x.IntervalNumbers
	}
	return nil
}

func (x *ExposureKeyExport) GetRollingPeriods() []byte {
	if x != nil {
		return x.RollingPeriods
	}
	return nil
}

var File_internal_pb_export_proto protoreflect.FileDescriptor

var file_internal_pb_export_proto_rawDesc = []byte{
	0x0a, 0x18, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x70, 0x62, 0x2f, 0x65, 0x78,
	0x70, 0x6f, 0x72, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xd5, 0x02, 0x0a, 0x11, 0x45,
	0x78, 0x70, 0x6f, 0x73, 0x75, 0x72, 0x65, 0x4b, 0x65, 0x79, 0x45, 0x78, 0x70, 0x6f, 0x72, 0x74,
	0x12, 0x26, 0x0a, 0x0e, 0x73, 0x74, 0x61, 0x72, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61,
	0x6d, 0x70, 0x18, 0x01, 0x20, 0x01, 0x28, 0x06, 0x52, 0x0e, 0x73, 0x74, 0x61, 0x72, 0x74, 0x54,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x22, 0x0a, 0x0c, 0x65, 0x6e, 0x64, 0x54,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x02, 0x20, 0x01, 0x28, 0x06, 0x52, 0x0c,
	0x65, 0x6e, 0x64, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x16, 0x0a, 0x06,
	0x72, 0x65, 0x67, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x72, 0x65,
	0x67, 0x69, 0x6f, 0x6e, 0x12, 0x1a, 0x0a, 0x08, 0x62, 0x61, 0x74, 0x63, 0x68, 0x4e, 0x75, 0x6d,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x62, 0x61, 0x74, 0x63, 0x68, 0x4e, 0x75, 0x6d,
	0x12, 0x1c, 0x0a, 0x09, 0x62, 0x61, 0x74, 0x63, 0x68, 0x53, 0x69, 0x7a, 0x65, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x05, 0x52, 0x09, 0x62, 0x61, 0x74, 0x63, 0x68, 0x53, 0x69, 0x7a, 0x65, 0x12, 0x22,
	0x0a, 0x0c, 0x65, 0x78, 0x70, 0x6f, 0x73, 0x75, 0x72, 0x65, 0x4b, 0x65, 0x79, 0x73, 0x18, 0x06,
	0x20, 0x01, 0x28, 0x0c, 0x52, 0x0c, 0x65, 0x78, 0x70, 0x6f, 0x73, 0x75, 0x72, 0x65, 0x4b, 0x65,
	0x79, 0x73, 0x12, 0x2c, 0x0a, 0x11, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x6d, 0x69, 0x73, 0x73, 0x69,
	0x6f, 0x6e, 0x52, 0x69, 0x73, 0x6b, 0x73, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x11, 0x74,
	0x72, 0x61, 0x6e, 0x73, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x69, 0x73, 0x6b, 0x73,
	0x12, 0x28, 0x0a, 0x0f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x4e, 0x75, 0x6d, 0x62,
	0x65, 0x72, 0x73, 0x18, 0x08, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0f, 0x69, 0x6e, 0x74, 0x65, 0x72,
	0x76, 0x61, 0x6c, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x73, 0x12, 0x26, 0x0a, 0x0e, 0x72, 0x6f,
	0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x50, 0x65, 0x72, 0x69, 0x6f, 0x64, 0x73, 0x18, 0x09, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x0e, 0x72, 0x6f, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x50, 0x65, 0x72, 0x69, 0x6f,
	0x64, 0x73, 0x42, 0x10, 0x5a, 0x0e, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x70,
	0x62, 0x3b, 0x70, 0x62,
}

var (
	file_internal_pb_export_proto_rawDescOnce sync.Once
	file_internal_pb_export_proto_rawDescData = file_internal_pb_export_proto_rawDesc
)

func file_internal_pb_export_proto_rawDescGZIP() []byte {
	file_internal_pb_export_proto_rawDescOnce.Do(func() {
		file_internal_pb_export_proto_rawDescData = protoimpl.X.CompressGZIP(file_internal_pb_export_proto_rawDescData)
	})
	return file_internal_pb_export_proto_rawDescData
}

var file_internal_pb_export_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_internal_pb_export_proto_goTypes = []interface{}{
	(*ExposureKeyExport)(nil), // 0: ExposureKeyExport
}
var file_internal_pb_export_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_internal_pb_export_proto_init() }
func file_internal_pb_export_proto_init() {
	if File_internal_pb_export_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_internal_pb_export_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ExposureKeyExport); i {
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
			RawDescriptor: file_internal_pb_export_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_internal_pb_export_proto_goTypes,
		DependencyIndexes: file_internal_pb_export_proto_depIdxs,
		MessageInfos:      file_internal_pb_export_proto_msgTypes,
	}.Build()
	File_internal_pb_export_proto = out.File
	file_internal_pb_export_proto_rawDesc = nil
	file_internal_pb_export_proto_goTypes = nil
	file_internal_pb_export_proto_depIdxs = nil
}
