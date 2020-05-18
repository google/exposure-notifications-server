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
// 	protoc-gen-go v1.22.0
// 	protoc        v3.11.4
// source: internal/pb/federation.proto

package pb

import (
	context "context"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
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

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type FederationFetchRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	FetchType                     string   `protobuf:"bytes,1,opt,name=fetchType,proto3" json:"fetchType,omitempty"` // required
	RegionIdentifiers             []string `protobuf:"bytes,2,rep,name=regionIdentifiers,proto3" json:"regionIdentifiers,omitempty"`
	ExcludeRegionIdentifiers      []string `protobuf:"bytes,3,rep,name=excludeRegionIdentifiers,proto3" json:"excludeRegionIdentifiers,omitempty"`
	LastFetchResponseKeyTimestamp int64    `protobuf:"varint,4,opt,name=lastFetchResponseKeyTimestamp,proto3" json:"lastFetchResponseKeyTimestamp,omitempty"` // required
	// regionIdentifiers, excludeRegionIdentifiers, lastFetchResponseKeyTimestamp must be stable to send a fetchToken.
	NextFetchToken string `protobuf:"bytes,5,opt,name=nextFetchToken,proto3" json:"nextFetchToken,omitempty"`
}

func (x *FederationFetchRequest) Reset() {
	*x = FederationFetchRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_pb_federation_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FederationFetchRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FederationFetchRequest) ProtoMessage() {}

func (x *FederationFetchRequest) ProtoReflect() protoreflect.Message {
	mi := &file_internal_pb_federation_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FederationFetchRequest.ProtoReflect.Descriptor instead.
func (*FederationFetchRequest) Descriptor() ([]byte, []int) {
	return file_internal_pb_federation_proto_rawDescGZIP(), []int{0}
}

func (x *FederationFetchRequest) GetFetchType() string {
	if x != nil {
		return x.FetchType
	}
	return ""
}

func (x *FederationFetchRequest) GetRegionIdentifiers() []string {
	if x != nil {
		return x.RegionIdentifiers
	}
	return nil
}

func (x *FederationFetchRequest) GetExcludeRegionIdentifiers() []string {
	if x != nil {
		return x.ExcludeRegionIdentifiers
	}
	return nil
}

func (x *FederationFetchRequest) GetLastFetchResponseKeyTimestamp() int64 {
	if x != nil {
		return x.LastFetchResponseKeyTimestamp
	}
	return 0
}

func (x *FederationFetchRequest) GetNextFetchToken() string {
	if x != nil {
		return x.NextFetchToken
	}
	return ""
}

type FederationFetchResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Response                  []*ContactTracingResponse `protobuf:"bytes,1,rep,name=response,proto3" json:"response,omitempty"`
	PartialResponse           bool                      `protobuf:"varint,2,opt,name=partialResponse,proto3" json:"partialResponse,omitempty"`                     // required
	NextFetchToken            string                    `protobuf:"bytes,3,opt,name=nextFetchToken,proto3" json:"nextFetchToken,omitempty"`                        // nextFetchToken will be present if partialResponse==true
	FetchResponseKeyTimestamp int64                     `protobuf:"varint,4,opt,name=fetchResponseKeyTimestamp,proto3" json:"fetchResponseKeyTimestamp,omitempty"` // required
}

func (x *FederationFetchResponse) Reset() {
	*x = FederationFetchResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_pb_federation_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FederationFetchResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FederationFetchResponse) ProtoMessage() {}

func (x *FederationFetchResponse) ProtoReflect() protoreflect.Message {
	mi := &file_internal_pb_federation_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FederationFetchResponse.ProtoReflect.Descriptor instead.
func (*FederationFetchResponse) Descriptor() ([]byte, []int) {
	return file_internal_pb_federation_proto_rawDescGZIP(), []int{1}
}

func (x *FederationFetchResponse) GetResponse() []*ContactTracingResponse {
	if x != nil {
		return x.Response
	}
	return nil
}

func (x *FederationFetchResponse) GetPartialResponse() bool {
	if x != nil {
		return x.PartialResponse
	}
	return false
}

func (x *FederationFetchResponse) GetNextFetchToken() string {
	if x != nil {
		return x.NextFetchToken
	}
	return ""
}

func (x *FederationFetchResponse) GetFetchResponseKeyTimestamp() int64 {
	if x != nil {
		return x.FetchResponseKeyTimestamp
	}
	return 0
}

type ContactTracingResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ContactTracingInfo []*ContactTracingInfo `protobuf:"bytes,1,rep,name=contactTracingInfo,proto3" json:"contactTracingInfo,omitempty"`
	RegionIdentifiers  []string              `protobuf:"bytes,2,rep,name=regionIdentifiers,proto3" json:"regionIdentifiers,omitempty"`
}

func (x *ContactTracingResponse) Reset() {
	*x = ContactTracingResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_pb_federation_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ContactTracingResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ContactTracingResponse) ProtoMessage() {}

func (x *ContactTracingResponse) ProtoReflect() protoreflect.Message {
	mi := &file_internal_pb_federation_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ContactTracingResponse.ProtoReflect.Descriptor instead.
func (*ContactTracingResponse) Descriptor() ([]byte, []int) {
	return file_internal_pb_federation_proto_rawDescGZIP(), []int{2}
}

func (x *ContactTracingResponse) GetContactTracingInfo() []*ContactTracingInfo {
	if x != nil {
		return x.ContactTracingInfo
	}
	return nil
}

func (x *ContactTracingResponse) GetRegionIdentifiers() []string {
	if x != nil {
		return x.RegionIdentifiers
	}
	return nil
}

type ContactTracingInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TransmissionRisk int32          `protobuf:"varint,1,opt,name=transmissionRisk,proto3" json:"transmissionRisk,omitempty"` // required
	ExposureKeys     []*ExposureKey `protobuf:"bytes,2,rep,name=exposureKeys,proto3" json:"exposureKeys,omitempty"`
}

func (x *ContactTracingInfo) Reset() {
	*x = ContactTracingInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_pb_federation_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ContactTracingInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ContactTracingInfo) ProtoMessage() {}

func (x *ContactTracingInfo) ProtoReflect() protoreflect.Message {
	mi := &file_internal_pb_federation_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ContactTracingInfo.ProtoReflect.Descriptor instead.
func (*ContactTracingInfo) Descriptor() ([]byte, []int) {
	return file_internal_pb_federation_proto_rawDescGZIP(), []int{3}
}

func (x *ContactTracingInfo) GetTransmissionRisk() int32 {
	if x != nil {
		return x.TransmissionRisk
	}
	return 0
}

func (x *ContactTracingInfo) GetExposureKeys() []*ExposureKey {
	if x != nil {
		return x.ExposureKeys
	}
	return nil
}

type ExposureKey struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ExposureKey    []byte `protobuf:"bytes,1,opt,name=exposureKey,proto3" json:"exposureKey,omitempty"`        // required
	IntervalNumber int32  `protobuf:"varint,2,opt,name=intervalNumber,proto3" json:"intervalNumber,omitempty"` // required
	IntervalCount  int32  `protobuf:"varint,3,opt,name=intervalCount,proto3" json:"intervalCount,omitempty"`   // required
}

func (x *ExposureKey) Reset() {
	*x = ExposureKey{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_pb_federation_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ExposureKey) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExposureKey) ProtoMessage() {}

func (x *ExposureKey) ProtoReflect() protoreflect.Message {
	mi := &file_internal_pb_federation_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExposureKey.ProtoReflect.Descriptor instead.
func (*ExposureKey) Descriptor() ([]byte, []int) {
	return file_internal_pb_federation_proto_rawDescGZIP(), []int{4}
}

func (x *ExposureKey) GetExposureKey() []byte {
	if x != nil {
		return x.ExposureKey
	}
	return nil
}

func (x *ExposureKey) GetIntervalNumber() int32 {
	if x != nil {
		return x.IntervalNumber
	}
	return 0
}

func (x *ExposureKey) GetIntervalCount() int32 {
	if x != nil {
		return x.IntervalCount
	}
	return 0
}

var File_internal_pb_federation_proto protoreflect.FileDescriptor

var file_internal_pb_federation_proto_rawDesc = []byte{
	0x0a, 0x1c, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x70, 0x62, 0x2f, 0x66, 0x65,
	0x64, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x8e,
	0x02, 0x0a, 0x16, 0x46, 0x65, 0x64, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x46, 0x65, 0x74,
	0x63, 0x68, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x66, 0x65, 0x74,
	0x63, 0x68, 0x54, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x66, 0x65,
	0x74, 0x63, 0x68, 0x54, 0x79, 0x70, 0x65, 0x12, 0x2c, 0x0a, 0x11, 0x72, 0x65, 0x67, 0x69, 0x6f,
	0x6e, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x65, 0x72, 0x73, 0x18, 0x02, 0x20, 0x03,
	0x28, 0x09, 0x52, 0x11, 0x72, 0x65, 0x67, 0x69, 0x6f, 0x6e, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69,
	0x66, 0x69, 0x65, 0x72, 0x73, 0x12, 0x3a, 0x0a, 0x18, 0x65, 0x78, 0x63, 0x6c, 0x75, 0x64, 0x65,
	0x52, 0x65, 0x67, 0x69, 0x6f, 0x6e, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x65, 0x72,
	0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x09, 0x52, 0x18, 0x65, 0x78, 0x63, 0x6c, 0x75, 0x64, 0x65,
	0x52, 0x65, 0x67, 0x69, 0x6f, 0x6e, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x65, 0x72,
	0x73, 0x12, 0x44, 0x0a, 0x1d, 0x6c, 0x61, 0x73, 0x74, 0x46, 0x65, 0x74, 0x63, 0x68, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x4b, 0x65, 0x79, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61,
	0x6d, 0x70, 0x18, 0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x1d, 0x6c, 0x61, 0x73, 0x74, 0x46, 0x65,
	0x74, 0x63, 0x68, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x4b, 0x65, 0x79, 0x54, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x26, 0x0a, 0x0e, 0x6e, 0x65, 0x78, 0x74, 0x46,
	0x65, 0x74, 0x63, 0x68, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0e, 0x6e, 0x65, 0x78, 0x74, 0x46, 0x65, 0x74, 0x63, 0x68, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x22,
	0xde, 0x01, 0x0a, 0x17, 0x46, 0x65, 0x64, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x46, 0x65,
	0x74, 0x63, 0x68, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x33, 0x0a, 0x08, 0x72,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x17, 0x2e,
	0x43, 0x6f, 0x6e, 0x74, 0x61, 0x63, 0x74, 0x54, 0x72, 0x61, 0x63, 0x69, 0x6e, 0x67, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x52, 0x08, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x28, 0x0a, 0x0f, 0x70, 0x61, 0x72, 0x74, 0x69, 0x61, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0f, 0x70, 0x61, 0x72, 0x74, 0x69,
	0x61, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x26, 0x0a, 0x0e, 0x6e, 0x65,
	0x78, 0x74, 0x46, 0x65, 0x74, 0x63, 0x68, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x0e, 0x6e, 0x65, 0x78, 0x74, 0x46, 0x65, 0x74, 0x63, 0x68, 0x54, 0x6f, 0x6b,
	0x65, 0x6e, 0x12, 0x3c, 0x0a, 0x19, 0x66, 0x65, 0x74, 0x63, 0x68, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x4b, 0x65, 0x79, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x19, 0x66, 0x65, 0x74, 0x63, 0x68, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x4b, 0x65, 0x79, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70,
	0x22, 0x8b, 0x01, 0x0a, 0x16, 0x43, 0x6f, 0x6e, 0x74, 0x61, 0x63, 0x74, 0x54, 0x72, 0x61, 0x63,
	0x69, 0x6e, 0x67, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x43, 0x0a, 0x12, 0x63,
	0x6f, 0x6e, 0x74, 0x61, 0x63, 0x74, 0x54, 0x72, 0x61, 0x63, 0x69, 0x6e, 0x67, 0x49, 0x6e, 0x66,
	0x6f, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x43, 0x6f, 0x6e, 0x74, 0x61, 0x63,
	0x74, 0x54, 0x72, 0x61, 0x63, 0x69, 0x6e, 0x67, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x12, 0x63, 0x6f,
	0x6e, 0x74, 0x61, 0x63, 0x74, 0x54, 0x72, 0x61, 0x63, 0x69, 0x6e, 0x67, 0x49, 0x6e, 0x66, 0x6f,
	0x12, 0x2c, 0x0a, 0x11, 0x72, 0x65, 0x67, 0x69, 0x6f, 0x6e, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69,
	0x66, 0x69, 0x65, 0x72, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x09, 0x52, 0x11, 0x72, 0x65, 0x67,
	0x69, 0x6f, 0x6e, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x65, 0x72, 0x73, 0x22, 0x72,
	0x0a, 0x12, 0x43, 0x6f, 0x6e, 0x74, 0x61, 0x63, 0x74, 0x54, 0x72, 0x61, 0x63, 0x69, 0x6e, 0x67,
	0x49, 0x6e, 0x66, 0x6f, 0x12, 0x2a, 0x0a, 0x10, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x6d, 0x69, 0x73,
	0x73, 0x69, 0x6f, 0x6e, 0x52, 0x69, 0x73, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x10,
	0x74, 0x72, 0x61, 0x6e, 0x73, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x69, 0x73, 0x6b,
	0x12, 0x30, 0x0a, 0x0c, 0x65, 0x78, 0x70, 0x6f, 0x73, 0x75, 0x72, 0x65, 0x4b, 0x65, 0x79, 0x73,
	0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0c, 0x2e, 0x45, 0x78, 0x70, 0x6f, 0x73, 0x75, 0x72,
	0x65, 0x4b, 0x65, 0x79, 0x52, 0x0c, 0x65, 0x78, 0x70, 0x6f, 0x73, 0x75, 0x72, 0x65, 0x4b, 0x65,
	0x79, 0x73, 0x22, 0x7d, 0x0a, 0x0b, 0x45, 0x78, 0x70, 0x6f, 0x73, 0x75, 0x72, 0x65, 0x4b, 0x65,
	0x79, 0x12, 0x20, 0x0a, 0x0b, 0x65, 0x78, 0x70, 0x6f, 0x73, 0x75, 0x72, 0x65, 0x4b, 0x65, 0x79,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0b, 0x65, 0x78, 0x70, 0x6f, 0x73, 0x75, 0x72, 0x65,
	0x4b, 0x65, 0x79, 0x12, 0x26, 0x0a, 0x0e, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x4e,
	0x75, 0x6d, 0x62, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0e, 0x69, 0x6e, 0x74,
	0x65, 0x72, 0x76, 0x61, 0x6c, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x12, 0x24, 0x0a, 0x0d, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x05, 0x52, 0x0d, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x43, 0x6f, 0x75, 0x6e,
	0x74, 0x32, 0x4a, 0x0a, 0x0a, 0x46, 0x65, 0x64, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12,
	0x3c, 0x0a, 0x05, 0x46, 0x65, 0x74, 0x63, 0x68, 0x12, 0x17, 0x2e, 0x46, 0x65, 0x64, 0x65, 0x72,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x46, 0x65, 0x74, 0x63, 0x68, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x18, 0x2e, 0x46, 0x65, 0x64, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x46, 0x65,
	0x74, 0x63, 0x68, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x10, 0x5a,
	0x0e, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x70, 0x62, 0x3b, 0x70, 0x62, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_internal_pb_federation_proto_rawDescOnce sync.Once
	file_internal_pb_federation_proto_rawDescData = file_internal_pb_federation_proto_rawDesc
)

func file_internal_pb_federation_proto_rawDescGZIP() []byte {
	file_internal_pb_federation_proto_rawDescOnce.Do(func() {
		file_internal_pb_federation_proto_rawDescData = protoimpl.X.CompressGZIP(file_internal_pb_federation_proto_rawDescData)
	})
	return file_internal_pb_federation_proto_rawDescData
}

var file_internal_pb_federation_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_internal_pb_federation_proto_goTypes = []interface{}{
	(*FederationFetchRequest)(nil),  // 0: FederationFetchRequest
	(*FederationFetchResponse)(nil), // 1: FederationFetchResponse
	(*ContactTracingResponse)(nil),  // 2: ContactTracingResponse
	(*ContactTracingInfo)(nil),      // 3: ContactTracingInfo
	(*ExposureKey)(nil),             // 4: ExposureKey
}
var file_internal_pb_federation_proto_depIdxs = []int32{
	2, // 0: FederationFetchResponse.response:type_name -> ContactTracingResponse
	3, // 1: ContactTracingResponse.contactTracingInfo:type_name -> ContactTracingInfo
	4, // 2: ContactTracingInfo.exposureKeys:type_name -> ExposureKey
	0, // 3: Federation.Fetch:input_type -> FederationFetchRequest
	1, // 4: Federation.Fetch:output_type -> FederationFetchResponse
	4, // [4:5] is the sub-list for method output_type
	3, // [3:4] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_internal_pb_federation_proto_init() }
func file_internal_pb_federation_proto_init() {
	if File_internal_pb_federation_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_internal_pb_federation_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FederationFetchRequest); i {
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
		file_internal_pb_federation_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FederationFetchResponse); i {
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
		file_internal_pb_federation_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ContactTracingResponse); i {
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
		file_internal_pb_federation_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ContactTracingInfo); i {
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
		file_internal_pb_federation_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ExposureKey); i {
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
			RawDescriptor: file_internal_pb_federation_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_internal_pb_federation_proto_goTypes,
		DependencyIndexes: file_internal_pb_federation_proto_depIdxs,
		MessageInfos:      file_internal_pb_federation_proto_msgTypes,
	}.Build()
	File_internal_pb_federation_proto = out.File
	file_internal_pb_federation_proto_rawDesc = nil
	file_internal_pb_federation_proto_goTypes = nil
	file_internal_pb_federation_proto_depIdxs = nil
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// FederationClient is the client API for Federation service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type FederationClient interface {
	Fetch(ctx context.Context, in *FederationFetchRequest, opts ...grpc.CallOption) (*FederationFetchResponse, error)
}

type federationClient struct {
	cc grpc.ClientConnInterface
}

func NewFederationClient(cc grpc.ClientConnInterface) FederationClient {
	return &federationClient{cc}
}

func (c *federationClient) Fetch(ctx context.Context, in *FederationFetchRequest, opts ...grpc.CallOption) (*FederationFetchResponse, error) {
	out := new(FederationFetchResponse)
	err := c.cc.Invoke(ctx, "/Federation/Fetch", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// FederationServer is the server API for Federation service.
type FederationServer interface {
	Fetch(context.Context, *FederationFetchRequest) (*FederationFetchResponse, error)
}

// UnimplementedFederationServer can be embedded to have forward compatible implementations.
type UnimplementedFederationServer struct {
}

func (*UnimplementedFederationServer) Fetch(context.Context, *FederationFetchRequest) (*FederationFetchResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Fetch not implemented")
}

func RegisterFederationServer(s *grpc.Server, srv FederationServer) {
	s.RegisterService(&_Federation_serviceDesc, srv)
}

func _Federation_Fetch_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(FederationFetchRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FederationServer).Fetch(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/Federation/Fetch",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FederationServer).Fetch(ctx, req.(*FederationFetchRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Federation_serviceDesc = grpc.ServiceDesc{
	ServiceName: "Federation",
	HandlerType: (*FederationServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Fetch",
			Handler:    _Federation_Fetch_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "internal/pb/federation.proto",
}
