// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: tx.proto

package pb

import (
	bytes "bytes"
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	io "io"
	math "math"
	math_bits "math/bits"
	redwood_dev_state "redwood.dev/state"
	pb "redwood.dev/state/pb"
	redwood_dev_types "redwood.dev/types"
	reflect "reflect"
	strconv "strconv"
	strings "strings"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type TxStatus int32

const (
	TxStatusUnknown   TxStatus = 0
	TxStatusInMempool TxStatus = 1
	TxStatusInvalid   TxStatus = 2
	TxStatusValid     TxStatus = 3
)

var TxStatus_name = map[int32]string{
	0: "Unknown",
	1: "InMempool",
	2: "Invalid",
	3: "Valid",
}

var TxStatus_value = map[string]int32{
	"Unknown":   0,
	"InMempool": 1,
	"Invalid":   2,
	"Valid":     3,
}

func (TxStatus) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_0fd2153dc07d3b5c, []int{0}
}

type Tx struct {
	ID         redwood_dev_state.Version   `protobuf:"bytes,1,opt,name=id,proto3,customtype=redwood.dev/state.Version" json:"id"`
	Parents    []redwood_dev_state.Version `protobuf:"bytes,2,rep,name=parents,proto3,customtype=redwood.dev/state.Version" json:"parents"`
	Children   []redwood_dev_state.Version `protobuf:"bytes,3,rep,name=children,proto3,customtype=redwood.dev/state.Version" json:"children"`
	From       redwood_dev_types.Address   `protobuf:"bytes,4,opt,name=from,proto3,customtype=redwood.dev/types.Address" json:"from"`
	Sig        redwood_dev_types.Signature `protobuf:"bytes,5,opt,name=sig,proto3,customtype=redwood.dev/types.Signature" json:"sig"`
	StateURI   string                      `protobuf:"bytes,6,opt,name=stateURI,proto3" json:"stateURI,omitempty"`
	Patches    []Patch                     `protobuf:"bytes,7,rep,name=patches,proto3" json:"patches"`
	Checkpoint bool                        `protobuf:"varint,8,opt,name=checkpoint,proto3" json:"checkpoint,omitempty"`
	Attachment []byte                      `protobuf:"bytes,9,opt,name=attachment,proto3" json:"attachment,omitempty"`
	Status     TxStatus                    `protobuf:"varint,10,opt,name=status,proto3,enum=Redwood.tree.TxStatus" json:"status,omitempty"`
}

func (m *Tx) Reset()      { *m = Tx{} }
func (*Tx) ProtoMessage() {}
func (*Tx) Descriptor() ([]byte, []int) {
	return fileDescriptor_0fd2153dc07d3b5c, []int{0}
}
func (m *Tx) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Tx) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Tx.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Tx) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Tx.Merge(m, src)
}
func (m *Tx) XXX_Size() int {
	return m.Size()
}
func (m *Tx) XXX_DiscardUnknown() {
	xxx_messageInfo_Tx.DiscardUnknown(m)
}

var xxx_messageInfo_Tx proto.InternalMessageInfo

func (m *Tx) GetStateURI() string {
	if m != nil {
		return m.StateURI
	}
	return ""
}

func (m *Tx) GetPatches() []Patch {
	if m != nil {
		return m.Patches
	}
	return nil
}

func (m *Tx) GetCheckpoint() bool {
	if m != nil {
		return m.Checkpoint
	}
	return false
}

func (m *Tx) GetAttachment() []byte {
	if m != nil {
		return m.Attachment
	}
	return nil
}

func (m *Tx) GetStatus() TxStatus {
	if m != nil {
		return m.Status
	}
	return TxStatusUnknown
}

type Patch struct {
	Keypath   redwood_dev_state.Keypath `protobuf:"bytes,1,opt,name=keypath,proto3,customtype=redwood.dev/state.Keypath" json:"keypath"`
	Range     *pb.Range                 `protobuf:"bytes,2,opt,name=range,proto3" json:"range,omitempty"`
	ValueJSON []byte                    `protobuf:"bytes,3,opt,name=valueJSON,proto3" json:"valueJSON,omitempty"`
}

func (m *Patch) Reset()      { *m = Patch{} }
func (*Patch) ProtoMessage() {}
func (*Patch) Descriptor() ([]byte, []int) {
	return fileDescriptor_0fd2153dc07d3b5c, []int{1}
}
func (m *Patch) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Patch) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Patch.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Patch) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Patch.Merge(m, src)
}
func (m *Patch) XXX_Size() int {
	return m.Size()
}
func (m *Patch) XXX_DiscardUnknown() {
	xxx_messageInfo_Patch.DiscardUnknown(m)
}

var xxx_messageInfo_Patch proto.InternalMessageInfo

func (m *Patch) GetRange() *pb.Range {
	if m != nil {
		return m.Range
	}
	return nil
}

func (m *Patch) GetValueJSON() []byte {
	if m != nil {
		return m.ValueJSON
	}
	return nil
}

func init() {
	proto.RegisterEnum("Redwood.tree.TxStatus", TxStatus_name, TxStatus_value)
	proto.RegisterType((*Tx)(nil), "Redwood.tree.Tx")
	proto.RegisterType((*Patch)(nil), "Redwood.tree.Patch")
}

func init() { proto.RegisterFile("tx.proto", fileDescriptor_0fd2153dc07d3b5c) }

var fileDescriptor_0fd2153dc07d3b5c = []byte{
	// 603 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x53, 0xbd, 0x6f, 0xd3, 0x4c,
	0x18, 0xf7, 0xd9, 0x49, 0xeb, 0x5c, 0xfb, 0xbe, 0xb4, 0xd7, 0x82, 0x4c, 0xa8, 0x2e, 0x47, 0x41,
	0xc2, 0xaa, 0x84, 0x23, 0xb5, 0xaa, 0x90, 0x8a, 0x18, 0x88, 0x58, 0x02, 0xe2, 0x43, 0xd7, 0x0f,
	0x09, 0x36, 0x27, 0xbe, 0x26, 0x56, 0x93, 0x3b, 0xcb, 0xbe, 0xf4, 0x63, 0xeb, 0xdc, 0x09, 0x31,
	0xb1, 0x54, 0x62, 0xe4, 0x4f, 0x60, 0x64, 0xec, 0xd8, 0xb1, 0x42, 0xa8, 0x6a, 0xdc, 0x85, 0x09,
	0x75, 0xec, 0x88, 0xee, 0x62, 0x87, 0x46, 0x20, 0x60, 0xbb, 0xe7, 0xf7, 0x71, 0xcf, 0xcf, 0xf7,
	0xf8, 0x81, 0xb6, 0xdc, 0xf5, 0xa2, 0x58, 0x48, 0x81, 0x26, 0x29, 0x0b, 0x76, 0x84, 0x08, 0x3c,
	0x19, 0x33, 0x56, 0xbe, 0xdf, 0x0a, 0x65, 0xbb, 0xd7, 0xf0, 0x9a, 0xa2, 0x5b, 0x6d, 0x89, 0x96,
	0xa8, 0x6a, 0x51, 0xa3, 0xb7, 0xa9, 0x2b, 0x5d, 0xe8, 0xd3, 0xc0, 0x5c, 0x9e, 0x4d, 0xa4, 0x2f,
	0x59, 0x35, 0x6a, 0x54, 0xf5, 0x61, 0x80, 0xce, 0x7f, 0xb7, 0xa0, 0xb9, 0xb6, 0x8b, 0x1e, 0x40,
	0x33, 0x0c, 0x1c, 0x40, 0x80, 0x3b, 0x59, 0xbb, 0x77, 0x74, 0x5a, 0x31, 0xbe, 0x9c, 0x56, 0x6e,
	0xc6, 0x59, 0xb7, 0x80, 0x6d, 0x67, 0x9e, 0x0d, 0x16, 0x27, 0xa1, 0xe0, 0xe9, 0x69, 0xc5, 0xac,
	0x3f, 0xa1, 0x66, 0x18, 0xa0, 0x87, 0x70, 0x3c, 0xf2, 0x63, 0xc6, 0x65, 0xe2, 0x98, 0xc4, 0x72,
	0x27, 0x6b, 0xb7, 0xff, 0xea, 0xa6, 0xb9, 0x03, 0x3d, 0x82, 0x76, 0xb3, 0x1d, 0x76, 0x82, 0x98,
	0x71, 0xc7, 0xfa, 0x57, 0xf7, 0xd0, 0x82, 0x96, 0x61, 0x61, 0x33, 0x16, 0x5d, 0xa7, 0xa0, 0x63,
	0xff, 0xd6, 0x2a, 0xf7, 0x22, 0x96, 0x78, 0x8f, 0x83, 0x20, 0x66, 0x49, 0x42, 0xb5, 0x1c, 0x2d,
	0x43, 0x2b, 0x09, 0x5b, 0x4e, 0x51, 0xbb, 0xee, 0x64, 0xae, 0x5b, 0xbf, 0xba, 0x56, 0xc3, 0x16,
	0xf7, 0x65, 0x2f, 0x66, 0x54, 0xe9, 0x51, 0x19, 0xda, 0x3a, 0xc8, 0x3a, 0xad, 0x3b, 0x63, 0x04,
	0xb8, 0x25, 0x3a, 0xac, 0xd1, 0x92, 0x7a, 0x05, 0xd9, 0x6c, 0xb3, 0xc4, 0x19, 0x27, 0x96, 0x3b,
	0xb1, 0x38, 0xe3, 0x5d, 0x1d, 0x95, 0xf7, 0x4a, 0x91, 0xb5, 0x82, 0xea, 0x45, 0x73, 0x25, 0xc2,
	0x10, 0x36, 0xdb, 0xac, 0xb9, 0x15, 0x89, 0x90, 0x4b, 0xc7, 0x26, 0xc0, 0xb5, 0xe9, 0x15, 0x44,
	0xf1, 0xbe, 0x94, 0x7e, 0xb3, 0xdd, 0x65, 0x5c, 0x3a, 0x25, 0x15, 0x97, 0x5e, 0x41, 0x90, 0x07,
	0xc7, 0x54, 0x80, 0x5e, 0xe2, 0x40, 0x02, 0xdc, 0xff, 0x17, 0x6f, 0x8c, 0xf6, 0x5c, 0xdb, 0x5d,
	0xd5, 0x2c, 0xcd, 0x54, 0x2b, 0x85, 0xcb, 0x0f, 0x15, 0x63, 0xfe, 0x3d, 0x80, 0x45, 0x1d, 0x47,
	0x8d, 0x6e, 0x8b, 0xed, 0x45, 0xbe, 0x6c, 0x67, 0x83, 0xff, 0xc3, 0xe3, 0x3f, 0x1b, 0x08, 0x69,
	0xee, 0x40, 0x0b, 0xb0, 0x18, 0xfb, 0xbc, 0xc5, 0x1c, 0x93, 0x00, 0x77, 0x62, 0x71, 0x76, 0xd8,
	0x7b, 0xa0, 0xa7, 0x8a, 0xa3, 0x03, 0x09, 0x9a, 0x83, 0xa5, 0x6d, 0xbf, 0xd3, 0x63, 0x4f, 0x57,
	0x5f, 0xbe, 0x70, 0x2c, 0xfd, 0x1d, 0x3f, 0x81, 0x15, 0x5b, 0xc5, 0xda, 0xff, 0x4a, 0x8c, 0x85,
	0x77, 0x00, 0xda, 0x79, 0x6a, 0x44, 0xe0, 0xf8, 0x3a, 0xdf, 0xe2, 0x62, 0x87, 0x4f, 0x19, 0xe5,
	0x99, 0x83, 0x43, 0x72, 0x2d, 0xa7, 0x32, 0x18, 0xdd, 0x85, 0xa5, 0x3a, 0x7f, 0xce, 0xba, 0x91,
	0x10, 0x9d, 0x29, 0x50, 0xbe, 0x7e, 0x70, 0x48, 0xa6, 0x73, 0xcd, 0x90, 0x50, 0xf7, 0xd4, 0xf9,
	0xb6, 0xdf, 0x09, 0x83, 0x29, 0x73, 0xf4, 0x9e, 0x0c, 0x46, 0x73, 0xb0, 0xb8, 0xa1, 0x79, 0xab,
	0x3c, 0x7d, 0x70, 0x48, 0xfe, 0xcb, 0x79, 0x0d, 0xd6, 0x5e, 0x1f, 0xf7, 0xb1, 0x71, 0xd2, 0xc7,
	0xc6, 0x59, 0x1f, 0x83, 0x8b, 0x3e, 0x06, 0x97, 0x7d, 0x0c, 0xf6, 0x53, 0x0c, 0x3e, 0xa6, 0x18,
	0x7c, 0x4a, 0x31, 0xf8, 0x9c, 0x62, 0x70, 0x94, 0x62, 0x70, 0x9c, 0x62, 0x70, 0x96, 0x62, 0xf0,
	0x2d, 0xc5, 0xc6, 0x45, 0x8a, 0xc1, 0xdb, 0x73, 0x6c, 0x1c, 0x9f, 0x63, 0xe3, 0xe4, 0x1c, 0x1b,
	0x6f, 0x66, 0x46, 0x7e, 0xaf, 0x98, 0xa9, 0x3d, 0x6c, 0x8c, 0xe9, 0x15, 0x5c, 0xfa, 0x11, 0x00,
	0x00, 0xff, 0xff, 0x8c, 0x71, 0x28, 0x2d, 0xe1, 0x03, 0x00, 0x00,
}

func (x TxStatus) String() string {
	s, ok := TxStatus_name[int32(x)]
	if ok {
		return s
	}
	return strconv.Itoa(int(x))
}
func (this *Tx) VerboseEqual(that interface{}) error {
	if that == nil {
		if this == nil {
			return nil
		}
		return fmt.Errorf("that == nil && this != nil")
	}

	that1, ok := that.(*Tx)
	if !ok {
		that2, ok := that.(Tx)
		if ok {
			that1 = &that2
		} else {
			return fmt.Errorf("that is not of type *Tx")
		}
	}
	if that1 == nil {
		if this == nil {
			return nil
		}
		return fmt.Errorf("that is type *Tx but is nil && this != nil")
	} else if this == nil {
		return fmt.Errorf("that is type *Tx but is not nil && this == nil")
	}
	if !this.ID.Equal(that1.ID) {
		return fmt.Errorf("ID this(%v) Not Equal that(%v)", this.ID, that1.ID)
	}
	if len(this.Parents) != len(that1.Parents) {
		return fmt.Errorf("Parents this(%v) Not Equal that(%v)", len(this.Parents), len(that1.Parents))
	}
	for i := range this.Parents {
		if !this.Parents[i].Equal(that1.Parents[i]) {
			return fmt.Errorf("Parents this[%v](%v) Not Equal that[%v](%v)", i, this.Parents[i], i, that1.Parents[i])
		}
	}
	if len(this.Children) != len(that1.Children) {
		return fmt.Errorf("Children this(%v) Not Equal that(%v)", len(this.Children), len(that1.Children))
	}
	for i := range this.Children {
		if !this.Children[i].Equal(that1.Children[i]) {
			return fmt.Errorf("Children this[%v](%v) Not Equal that[%v](%v)", i, this.Children[i], i, that1.Children[i])
		}
	}
	if !this.From.Equal(that1.From) {
		return fmt.Errorf("From this(%v) Not Equal that(%v)", this.From, that1.From)
	}
	if !this.Sig.Equal(that1.Sig) {
		return fmt.Errorf("Sig this(%v) Not Equal that(%v)", this.Sig, that1.Sig)
	}
	if this.StateURI != that1.StateURI {
		return fmt.Errorf("StateURI this(%v) Not Equal that(%v)", this.StateURI, that1.StateURI)
	}
	if len(this.Patches) != len(that1.Patches) {
		return fmt.Errorf("Patches this(%v) Not Equal that(%v)", len(this.Patches), len(that1.Patches))
	}
	for i := range this.Patches {
		if !this.Patches[i].Equal(&that1.Patches[i]) {
			return fmt.Errorf("Patches this[%v](%v) Not Equal that[%v](%v)", i, this.Patches[i], i, that1.Patches[i])
		}
	}
	if this.Checkpoint != that1.Checkpoint {
		return fmt.Errorf("Checkpoint this(%v) Not Equal that(%v)", this.Checkpoint, that1.Checkpoint)
	}
	if !bytes.Equal(this.Attachment, that1.Attachment) {
		return fmt.Errorf("Attachment this(%v) Not Equal that(%v)", this.Attachment, that1.Attachment)
	}
	if this.Status != that1.Status {
		return fmt.Errorf("Status this(%v) Not Equal that(%v)", this.Status, that1.Status)
	}
	return nil
}
func (this *Tx) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Tx)
	if !ok {
		that2, ok := that.(Tx)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if !this.ID.Equal(that1.ID) {
		return false
	}
	if len(this.Parents) != len(that1.Parents) {
		return false
	}
	for i := range this.Parents {
		if !this.Parents[i].Equal(that1.Parents[i]) {
			return false
		}
	}
	if len(this.Children) != len(that1.Children) {
		return false
	}
	for i := range this.Children {
		if !this.Children[i].Equal(that1.Children[i]) {
			return false
		}
	}
	if !this.From.Equal(that1.From) {
		return false
	}
	if !this.Sig.Equal(that1.Sig) {
		return false
	}
	if this.StateURI != that1.StateURI {
		return false
	}
	if len(this.Patches) != len(that1.Patches) {
		return false
	}
	for i := range this.Patches {
		if !this.Patches[i].Equal(&that1.Patches[i]) {
			return false
		}
	}
	if this.Checkpoint != that1.Checkpoint {
		return false
	}
	if !bytes.Equal(this.Attachment, that1.Attachment) {
		return false
	}
	if this.Status != that1.Status {
		return false
	}
	return true
}
func (this *Patch) VerboseEqual(that interface{}) error {
	if that == nil {
		if this == nil {
			return nil
		}
		return fmt.Errorf("that == nil && this != nil")
	}

	that1, ok := that.(*Patch)
	if !ok {
		that2, ok := that.(Patch)
		if ok {
			that1 = &that2
		} else {
			return fmt.Errorf("that is not of type *Patch")
		}
	}
	if that1 == nil {
		if this == nil {
			return nil
		}
		return fmt.Errorf("that is type *Patch but is nil && this != nil")
	} else if this == nil {
		return fmt.Errorf("that is type *Patch but is not nil && this == nil")
	}
	if !this.Keypath.Equal(that1.Keypath) {
		return fmt.Errorf("Keypath this(%v) Not Equal that(%v)", this.Keypath, that1.Keypath)
	}
	if !this.Range.Equal(that1.Range) {
		return fmt.Errorf("Range this(%v) Not Equal that(%v)", this.Range, that1.Range)
	}
	if !bytes.Equal(this.ValueJSON, that1.ValueJSON) {
		return fmt.Errorf("ValueJSON this(%v) Not Equal that(%v)", this.ValueJSON, that1.ValueJSON)
	}
	return nil
}
func (this *Patch) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Patch)
	if !ok {
		that2, ok := that.(Patch)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if !this.Keypath.Equal(that1.Keypath) {
		return false
	}
	if !this.Range.Equal(that1.Range) {
		return false
	}
	if !bytes.Equal(this.ValueJSON, that1.ValueJSON) {
		return false
	}
	return true
}
func (this *Tx) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 14)
	s = append(s, "&pb.Tx{")
	s = append(s, "ID: "+fmt.Sprintf("%#v", this.ID)+",\n")
	s = append(s, "Parents: "+fmt.Sprintf("%#v", this.Parents)+",\n")
	s = append(s, "Children: "+fmt.Sprintf("%#v", this.Children)+",\n")
	s = append(s, "From: "+fmt.Sprintf("%#v", this.From)+",\n")
	s = append(s, "Sig: "+fmt.Sprintf("%#v", this.Sig)+",\n")
	s = append(s, "StateURI: "+fmt.Sprintf("%#v", this.StateURI)+",\n")
	if this.Patches != nil {
		vs := make([]Patch, len(this.Patches))
		for i := range vs {
			vs[i] = this.Patches[i]
		}
		s = append(s, "Patches: "+fmt.Sprintf("%#v", vs)+",\n")
	}
	s = append(s, "Checkpoint: "+fmt.Sprintf("%#v", this.Checkpoint)+",\n")
	s = append(s, "Attachment: "+fmt.Sprintf("%#v", this.Attachment)+",\n")
	s = append(s, "Status: "+fmt.Sprintf("%#v", this.Status)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *Patch) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 7)
	s = append(s, "&pb.Patch{")
	s = append(s, "Keypath: "+fmt.Sprintf("%#v", this.Keypath)+",\n")
	if this.Range != nil {
		s = append(s, "Range: "+fmt.Sprintf("%#v", this.Range)+",\n")
	}
	s = append(s, "ValueJSON: "+fmt.Sprintf("%#v", this.ValueJSON)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func valueToGoStringTx(v interface{}, typ string) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("func(v %v) *%v { return &v } ( %#v )", typ, typ, pv)
}
func (m *Tx) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Tx) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Tx) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Status != 0 {
		i = encodeVarintTx(dAtA, i, uint64(m.Status))
		i--
		dAtA[i] = 0x50
	}
	if len(m.Attachment) > 0 {
		i -= len(m.Attachment)
		copy(dAtA[i:], m.Attachment)
		i = encodeVarintTx(dAtA, i, uint64(len(m.Attachment)))
		i--
		dAtA[i] = 0x4a
	}
	if m.Checkpoint {
		i--
		if m.Checkpoint {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x40
	}
	if len(m.Patches) > 0 {
		for iNdEx := len(m.Patches) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Patches[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintTx(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x3a
		}
	}
	if len(m.StateURI) > 0 {
		i -= len(m.StateURI)
		copy(dAtA[i:], m.StateURI)
		i = encodeVarintTx(dAtA, i, uint64(len(m.StateURI)))
		i--
		dAtA[i] = 0x32
	}
	{
		size := m.Sig.Size()
		i -= size
		if _, err := m.Sig.MarshalTo(dAtA[i:]); err != nil {
			return 0, err
		}
		i = encodeVarintTx(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0x2a
	{
		size := m.From.Size()
		i -= size
		if _, err := m.From.MarshalTo(dAtA[i:]); err != nil {
			return 0, err
		}
		i = encodeVarintTx(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0x22
	if len(m.Children) > 0 {
		for iNdEx := len(m.Children) - 1; iNdEx >= 0; iNdEx-- {
			{
				size := m.Children[iNdEx].Size()
				i -= size
				if _, err := m.Children[iNdEx].MarshalTo(dAtA[i:]); err != nil {
					return 0, err
				}
				i = encodeVarintTx(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x1a
		}
	}
	if len(m.Parents) > 0 {
		for iNdEx := len(m.Parents) - 1; iNdEx >= 0; iNdEx-- {
			{
				size := m.Parents[iNdEx].Size()
				i -= size
				if _, err := m.Parents[iNdEx].MarshalTo(dAtA[i:]); err != nil {
					return 0, err
				}
				i = encodeVarintTx(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x12
		}
	}
	{
		size := m.ID.Size()
		i -= size
		if _, err := m.ID.MarshalTo(dAtA[i:]); err != nil {
			return 0, err
		}
		i = encodeVarintTx(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0xa
	return len(dAtA) - i, nil
}

func (m *Patch) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Patch) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Patch) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.ValueJSON) > 0 {
		i -= len(m.ValueJSON)
		copy(dAtA[i:], m.ValueJSON)
		i = encodeVarintTx(dAtA, i, uint64(len(m.ValueJSON)))
		i--
		dAtA[i] = 0x1a
	}
	if m.Range != nil {
		{
			size, err := m.Range.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintTx(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x12
	}
	{
		size := m.Keypath.Size()
		i -= size
		if _, err := m.Keypath.MarshalTo(dAtA[i:]); err != nil {
			return 0, err
		}
		i = encodeVarintTx(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0xa
	return len(dAtA) - i, nil
}

func encodeVarintTx(dAtA []byte, offset int, v uint64) int {
	offset -= sovTx(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func NewPopulatedTx(r randyTx, easy bool) *Tx {
	this := &Tx{}
	v1 := redwood_dev_state.NewPopulatedVersion(r)
	this.ID = *v1
	v2 := r.Intn(10)
	this.Parents = make([]redwood_dev_state.Version, v2)
	for i := 0; i < v2; i++ {
		v3 := redwood_dev_state.NewPopulatedVersion(r)
		this.Parents[i] = *v3
	}
	v4 := r.Intn(10)
	this.Children = make([]redwood_dev_state.Version, v4)
	for i := 0; i < v4; i++ {
		v5 := redwood_dev_state.NewPopulatedVersion(r)
		this.Children[i] = *v5
	}
	v6 := redwood_dev_types.NewPopulatedAddress(r)
	this.From = *v6
	v7 := redwood_dev_types.NewPopulatedSignature(r)
	this.Sig = *v7
	this.StateURI = string(randStringTx(r))
	if r.Intn(5) != 0 {
		v8 := r.Intn(5)
		this.Patches = make([]Patch, v8)
		for i := 0; i < v8; i++ {
			v9 := NewPopulatedPatch(r, easy)
			this.Patches[i] = *v9
		}
	}
	this.Checkpoint = bool(bool(r.Intn(2) == 0))
	v10 := r.Intn(100)
	this.Attachment = make([]byte, v10)
	for i := 0; i < v10; i++ {
		this.Attachment[i] = byte(r.Intn(256))
	}
	this.Status = TxStatus([]int32{0, 1, 2, 3}[r.Intn(4)])
	if !easy && r.Intn(10) != 0 {
	}
	return this
}

func NewPopulatedPatch(r randyTx, easy bool) *Patch {
	this := &Patch{}
	v11 := redwood_dev_state.NewPopulatedKeypath(r)
	this.Keypath = *v11
	if r.Intn(5) != 0 {
		this.Range = pb.NewPopulatedRange(r, easy)
	}
	v12 := r.Intn(100)
	this.ValueJSON = make([]byte, v12)
	for i := 0; i < v12; i++ {
		this.ValueJSON[i] = byte(r.Intn(256))
	}
	if !easy && r.Intn(10) != 0 {
	}
	return this
}

type randyTx interface {
	Float32() float32
	Float64() float64
	Int63() int64
	Int31() int32
	Uint32() uint32
	Intn(n int) int
}

func randUTF8RuneTx(r randyTx) rune {
	ru := r.Intn(62)
	if ru < 10 {
		return rune(ru + 48)
	} else if ru < 36 {
		return rune(ru + 55)
	}
	return rune(ru + 61)
}
func randStringTx(r randyTx) string {
	v13 := r.Intn(100)
	tmps := make([]rune, v13)
	for i := 0; i < v13; i++ {
		tmps[i] = randUTF8RuneTx(r)
	}
	return string(tmps)
}
func randUnrecognizedTx(r randyTx, maxFieldNumber int) (dAtA []byte) {
	l := r.Intn(5)
	for i := 0; i < l; i++ {
		wire := r.Intn(4)
		if wire == 3 {
			wire = 5
		}
		fieldNumber := maxFieldNumber + r.Intn(100)
		dAtA = randFieldTx(dAtA, r, fieldNumber, wire)
	}
	return dAtA
}
func randFieldTx(dAtA []byte, r randyTx, fieldNumber int, wire int) []byte {
	key := uint32(fieldNumber)<<3 | uint32(wire)
	switch wire {
	case 0:
		dAtA = encodeVarintPopulateTx(dAtA, uint64(key))
		v14 := r.Int63()
		if r.Intn(2) == 0 {
			v14 *= -1
		}
		dAtA = encodeVarintPopulateTx(dAtA, uint64(v14))
	case 1:
		dAtA = encodeVarintPopulateTx(dAtA, uint64(key))
		dAtA = append(dAtA, byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)))
	case 2:
		dAtA = encodeVarintPopulateTx(dAtA, uint64(key))
		ll := r.Intn(100)
		dAtA = encodeVarintPopulateTx(dAtA, uint64(ll))
		for j := 0; j < ll; j++ {
			dAtA = append(dAtA, byte(r.Intn(256)))
		}
	default:
		dAtA = encodeVarintPopulateTx(dAtA, uint64(key))
		dAtA = append(dAtA, byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)))
	}
	return dAtA
}
func encodeVarintPopulateTx(dAtA []byte, v uint64) []byte {
	for v >= 1<<7 {
		dAtA = append(dAtA, uint8(uint64(v)&0x7f|0x80))
		v >>= 7
	}
	dAtA = append(dAtA, uint8(v))
	return dAtA
}
func (m *Tx) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = m.ID.Size()
	n += 1 + l + sovTx(uint64(l))
	if len(m.Parents) > 0 {
		for _, e := range m.Parents {
			l = e.Size()
			n += 1 + l + sovTx(uint64(l))
		}
	}
	if len(m.Children) > 0 {
		for _, e := range m.Children {
			l = e.Size()
			n += 1 + l + sovTx(uint64(l))
		}
	}
	l = m.From.Size()
	n += 1 + l + sovTx(uint64(l))
	l = m.Sig.Size()
	n += 1 + l + sovTx(uint64(l))
	l = len(m.StateURI)
	if l > 0 {
		n += 1 + l + sovTx(uint64(l))
	}
	if len(m.Patches) > 0 {
		for _, e := range m.Patches {
			l = e.Size()
			n += 1 + l + sovTx(uint64(l))
		}
	}
	if m.Checkpoint {
		n += 2
	}
	l = len(m.Attachment)
	if l > 0 {
		n += 1 + l + sovTx(uint64(l))
	}
	if m.Status != 0 {
		n += 1 + sovTx(uint64(m.Status))
	}
	return n
}

func (m *Patch) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = m.Keypath.Size()
	n += 1 + l + sovTx(uint64(l))
	if m.Range != nil {
		l = m.Range.Size()
		n += 1 + l + sovTx(uint64(l))
	}
	l = len(m.ValueJSON)
	if l > 0 {
		n += 1 + l + sovTx(uint64(l))
	}
	return n
}

func sovTx(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozTx(x uint64) (n int) {
	return sovTx(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *Tx) String() string {
	if this == nil {
		return "nil"
	}
	repeatedStringForPatches := "[]Patch{"
	for _, f := range this.Patches {
		repeatedStringForPatches += fmt.Sprintf("%v", f) + ","
	}
	repeatedStringForPatches += "}"
	s := strings.Join([]string{`&Tx{`,
		`ID:` + fmt.Sprintf("%v", this.ID) + `,`,
		`Parents:` + fmt.Sprintf("%v", this.Parents) + `,`,
		`Children:` + fmt.Sprintf("%v", this.Children) + `,`,
		`From:` + fmt.Sprintf("%v", this.From) + `,`,
		`Sig:` + fmt.Sprintf("%v", this.Sig) + `,`,
		`StateURI:` + fmt.Sprintf("%v", this.StateURI) + `,`,
		`Patches:` + repeatedStringForPatches + `,`,
		`Checkpoint:` + fmt.Sprintf("%v", this.Checkpoint) + `,`,
		`Attachment:` + fmt.Sprintf("%v", this.Attachment) + `,`,
		`Status:` + fmt.Sprintf("%v", this.Status) + `,`,
		`}`,
	}, "")
	return s
}
func valueToStringTx(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
func (m *Tx) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTx
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Tx: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Tx: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ID", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.ID.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Parents", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			var v redwood_dev_state.Version
			m.Parents = append(m.Parents, v)
			if err := m.Parents[len(m.Parents)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Children", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			var v redwood_dev_state.Version
			m.Children = append(m.Children, v)
			if err := m.Children[len(m.Children)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field From", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.From.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Sig", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Sig.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 6:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field StateURI", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.StateURI = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 7:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Patches", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Patches = append(m.Patches, Patch{})
			if err := m.Patches[len(m.Patches)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 8:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Checkpoint", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Checkpoint = bool(v != 0)
		case 9:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Attachment", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Attachment = append(m.Attachment[:0], dAtA[iNdEx:postIndex]...)
			if m.Attachment == nil {
				m.Attachment = []byte{}
			}
			iNdEx = postIndex
		case 10:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Status", wireType)
			}
			m.Status = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Status |= TxStatus(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipTx(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthTx
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Patch) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTx
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Patch: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Patch: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Keypath", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Keypath.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Range", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Range == nil {
				m.Range = &pb.Range{}
			}
			if err := m.Range.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ValueJSON", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ValueJSON = append(m.ValueJSON[:0], dAtA[iNdEx:postIndex]...)
			if m.ValueJSON == nil {
				m.ValueJSON = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipTx(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthTx
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipTx(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowTx
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowTx
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowTx
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthTx
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupTx
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthTx
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthTx        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowTx          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupTx = fmt.Errorf("proto: unexpected end of group")
)
