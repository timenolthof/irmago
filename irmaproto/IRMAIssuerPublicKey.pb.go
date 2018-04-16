// Code generated by protoc-gen-go. DO NOT EDIT.
// source: IRMAIssuerPublicKey.proto

package irmaproto

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This mimicks IRMA issuer description.xml
type IRMAIssuerPublicKey struct {
	Counter     int32    `protobuf:"varint,1,opt,name=Counter" json:"Counter,omitempty"`
	ExpiryDate  int64    `protobuf:"varint,2,opt,name=ExpiryDate" json:"ExpiryDate,omitempty"`
	N           []byte   `protobuf:"bytes,3,opt,name=N,proto3" json:"N,omitempty"`
	Z           []byte   `protobuf:"bytes,4,opt,name=Z,proto3" json:"Z,omitempty"`
	S           []byte   `protobuf:"bytes,5,opt,name=S,proto3" json:"S,omitempty"`
	Bases       [][]byte `protobuf:"bytes,6,rep,name=Bases,proto3" json:"Bases,omitempty"`
	EpochLength int32    `protobuf:"varint,7,opt,name=EpochLength" json:"EpochLength,omitempty"`
}

func (m *IRMAIssuerPublicKey) Reset()                    { *m = IRMAIssuerPublicKey{} }
func (m *IRMAIssuerPublicKey) String() string            { return proto.CompactTextString(m) }
func (*IRMAIssuerPublicKey) ProtoMessage()               {}
func (*IRMAIssuerPublicKey) Descriptor() ([]byte, []int) { return fileDescriptor2, []int{0} }

func (m *IRMAIssuerPublicKey) GetCounter() int32 {
	if m != nil {
		return m.Counter
	}
	return 0
}

func (m *IRMAIssuerPublicKey) GetExpiryDate() int64 {
	if m != nil {
		return m.ExpiryDate
	}
	return 0
}

func (m *IRMAIssuerPublicKey) GetN() []byte {
	if m != nil {
		return m.N
	}
	return nil
}

func (m *IRMAIssuerPublicKey) GetZ() []byte {
	if m != nil {
		return m.Z
	}
	return nil
}

func (m *IRMAIssuerPublicKey) GetS() []byte {
	if m != nil {
		return m.S
	}
	return nil
}

func (m *IRMAIssuerPublicKey) GetBases() [][]byte {
	if m != nil {
		return m.Bases
	}
	return nil
}

func (m *IRMAIssuerPublicKey) GetEpochLength() int32 {
	if m != nil {
		return m.EpochLength
	}
	return 0
}

func init() {
	proto.RegisterType((*IRMAIssuerPublicKey)(nil), "irmaproto.IRMAIssuerPublicKey")
}

func init() { proto.RegisterFile("IRMAIssuerPublicKey.proto", fileDescriptor2) }

var fileDescriptor2 = []byte{
	// 182 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x92, 0xf4, 0x0c, 0xf2, 0x75,
	0xf4, 0x2c, 0x2e, 0x2e, 0x4d, 0x2d, 0x0a, 0x28, 0x4d, 0xca, 0xc9, 0x4c, 0xf6, 0x4e, 0xad, 0xd4,
	0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2, 0xcc, 0x2c, 0xca, 0x4d, 0x04, 0x33, 0x95, 0x36, 0x32,
	0x72, 0x09, 0x63, 0x51, 0x28, 0x24, 0xc1, 0xc5, 0xee, 0x9c, 0x5f, 0x9a, 0x57, 0x92, 0x5a, 0x24,
	0xc1, 0xa8, 0xc0, 0xa8, 0xc1, 0x1a, 0x04, 0xe3, 0x0a, 0xc9, 0x71, 0x71, 0xb9, 0x56, 0x14, 0x64,
	0x16, 0x55, 0xba, 0x24, 0x96, 0xa4, 0x4a, 0x30, 0x29, 0x30, 0x6a, 0x30, 0x07, 0x21, 0x89, 0x08,
	0xf1, 0x70, 0x31, 0xfa, 0x49, 0x30, 0x2b, 0x30, 0x6a, 0xf0, 0x04, 0x31, 0xfa, 0x81, 0x78, 0x51,
	0x12, 0x2c, 0x10, 0x5e, 0x14, 0x88, 0x17, 0x2c, 0xc1, 0x0a, 0xe1, 0x05, 0x0b, 0x89, 0x70, 0xb1,
	0x3a, 0x25, 0x16, 0xa7, 0x16, 0x4b, 0xb0, 0x29, 0x30, 0x6b, 0xf0, 0x04, 0x41, 0x38, 0x42, 0x0a,
	0x5c, 0xdc, 0xae, 0x05, 0xf9, 0xc9, 0x19, 0x3e, 0xa9, 0x79, 0xe9, 0x25, 0x19, 0x12, 0xec, 0x60,
	0xdb, 0x91, 0x85, 0x92, 0xd8, 0xc0, 0x4e, 0x37, 0x06, 0x04, 0x00, 0x00, 0xff, 0xff, 0x81, 0x7e,
	0x71, 0x1a, 0xe2, 0x00, 0x00, 0x00,
}
