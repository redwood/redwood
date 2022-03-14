package nelson

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/url"
	"strings"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/types"
)

type Node interface {
	state.Node
	ContentType() string
	ContentLength() (int64, error)
	BytesReader(rng *types.Range) (io.ReadCloser, int64, error)
}

type Frame struct {
	state.Node
	contentType string
	linkType    LinkType
	linkValue   string
}

type BlobFrame struct {
	Frame
	blobID   blob.ID
	resolver Resolver
}

type HTTPFrame struct {
	Frame
	url      url.URL
	resolver Resolver
}

var _ Node = Frame{}
var _ Node = BlobFrame{}
var _ Node = HTTPFrame{}

var (
	ValueKey       = state.Keypath("value")
	ContentTypeKey = state.Keypath("Content-Type")
)

func (frame Frame) ContentType() string {
	return frame.contentType
}

func (frame Frame) ContentLength() (int64, error) {
	_, contentLength, err := frame.BytesReader(nil)
	if err != nil {
		return 0, err
	}
	return contentLength, nil
}

func (frame Frame) Value(keypath state.Keypath, rng *state.Range) (interface{}, bool, error) {
	return frame.Node.Value(keypath, rng)
}

func (frame Frame) CopyToMemory(keypath state.Keypath, rng *state.Range) (state.Node, error) {
	copiedNode, err := frame.Node.CopyToMemory(keypath, rng)
	if err != nil {
		return nil, err
	}
	return Frame{
		Node:        copiedNode,
		contentType: frame.contentType,
		linkType:    frame.linkType,
		linkValue:   frame.linkValue,
	}, nil
}

func (f Frame) BytesReader(rng *types.Range) (io.ReadCloser, int64, error) {
	bs, err := json.Marshal(f.Node)
	if err != nil {
		return nil, 0, err
	}
	if rng != nil {
		bs = bs[rng.Start:rng.End]
	}
	return ioutil.NopCloser(bytes.NewReader(bs)), int64(len(bs)), nil
}

func (frame Frame) NodeAt(keypath state.Keypath, rng *state.Range) state.Node {
	if len(keypath) == 0 && rng == nil {
		return frame
	}
	return frame.Node.NodeAt(keypath, rng)
}

func (frame Frame) ParentNodeFor(keypath state.Keypath) (state.Node, state.Keypath) {
	// This is necessary -- otherwise, fetching a Frame node will actually
	// return the underlying state.Node
	if len(keypath) == 0 {
		return frame, nil
	}
	_, keypath = keypath.Shift()
	parent, relKeypath := frame.Node.ParentNodeFor(keypath)
	if parent == frame.Node {
		return frame, relKeypath
	} else {
		return parent, relKeypath
	}
}

func (frame Frame) MarshalJSON() ([]byte, error) {
	return json.Marshal(frame.Node)
}

func (frame Frame) DebugPrint(printFn func(inFormat string, args ...interface{}), newlines bool, indentLevel int) {
	if newlines {
		oldPrintFn := printFn
		printFn = func(inFormat string, args ...interface{}) { oldPrintFn(inFormat+"\n", args...) }
	}

	indent := strings.Repeat(" ", 4*indentLevel)

	printFn(indent + "NelSON Frame {")
	printFn(indent+"    [contentType: %v]", frame.contentType)
	printFn(indent+"    [linkType: %v]", frame.linkType)
	printFn(indent+"    [linkValue: %v]", frame.linkValue)
	if frame.Node != nil {
		frame.Node.DebugPrint(printFn, false, indentLevel+1)
	}
	printFn(indent + "}")
}

func (frame BlobFrame) NodeAt(keypath state.Keypath, rng *state.Range) state.Node {
	if len(keypath) == 0 && rng == nil {
		return frame
	}
	return frame.Node.NodeAt(keypath, rng)
}

func (frame BlobFrame) ParentNodeFor(keypath state.Keypath) (state.Node, state.Keypath) {
	// This is necessary -- otherwise, fetching a Frame node will actually
	// return the underlying state.Node
	if len(keypath) == 0 {
		return frame, nil
	}
	_, keypath = keypath.Shift()
	parent, relKeypath := frame.Node.ParentNodeFor(keypath)
	if parent == frame.Node {
		return frame, relKeypath
	} else {
		return parent, relKeypath
	}
}

func (frame BlobFrame) ContentLength() (int64, error) {
	manifest, err := frame.resolver.blobResolver.Manifest(frame.blobID)
	if err != nil {
		return 0, err
	}
	return int64(manifest.TotalSize), nil
}

func (frame BlobFrame) BytesReader(rng *types.Range) (io.ReadCloser, int64, error) {
	return frame.resolver.blobResolver.BlobReader(frame.blobID, rng)
}

func (frame BlobFrame) BytesValue(keypath state.Keypath) ([]byte, bool, error) {
	if len(keypath) > 0 {
		return nil, false, errors.Err404
	}
	reader, _, err := frame.resolver.blobResolver.BlobReader(frame.blobID, nil)
	if err != nil {
		return nil, false, err
	}
	defer reader.Close()
	bs, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, false, err
	}
	return bs, true, nil
}

func (f BlobFrame) StringValue(keypath state.Keypath) (string, bool, error) {
	bs, _, err := f.BytesValue(keypath)
	if err != nil {
		return "", false, err
	}
	return string(bs), true, nil
}

func (frame BlobFrame) MarshalJSON() ([]byte, error) {
	bs, _, err := frame.BytesValue(nil)
	if err != nil {
		return nil, err
	}
	return []byte(`"` + hex.EncodeToString(bs) + `"`), nil
}

func (frame HTTPFrame) NodeAt(keypath state.Keypath, rng *state.Range) state.Node {
	if len(keypath) == 0 && rng == nil {
		return frame
	}
	return frame.Node.NodeAt(keypath, rng)
}

func (frame HTTPFrame) ParentNodeFor(keypath state.Keypath) (state.Node, state.Keypath) {
	// This is necessary -- otherwise, fetching a Frame node will actually
	// return the underlying state.Node
	if len(keypath) == 0 {
		return frame, nil
	}
	_, keypath = keypath.Shift()
	parent, relKeypath := frame.Node.ParentNodeFor(keypath)
	if parent == frame.Node {
		return frame, relKeypath
	} else {
		return parent, relKeypath
	}
}

func (frame HTTPFrame) ContentLength() (int64, error) {
	_, _, contentLength, err := frame.resolver.httpResolver.Metadata(frame.url)
	if err != nil {
		return 0, err
	}
	return contentLength, nil
}

func (frame HTTPFrame) BytesReader(rng *types.Range) (io.ReadCloser, int64, error) {
	reader, _, contentLength, err := frame.resolver.httpResolver.Get(frame.url, rng)
	return reader, contentLength, err
}

func (frame HTTPFrame) BytesValue(keypath state.Keypath) ([]byte, bool, error) {
	if len(keypath) > 0 {
		return nil, false, errors.Err404
	}
	reader, _, _, err := frame.resolver.httpResolver.Get(frame.url, nil)
	if err != nil {
		return nil, false, err
	}
	defer reader.Close()
	bs, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, false, err
	}
	return bs, true, nil
}

func (frame HTTPFrame) StringValue(keypath state.Keypath) (string, bool, error) {
	bs, _, err := frame.BytesValue(keypath)
	if err != nil {
		return "", false, err
	}
	return string(bs), true, nil
}

func (frame HTTPFrame) MarshalJSON() ([]byte, error) {
	bs, _, err := frame.BytesValue(nil)
	if err != nil {
		return nil, err
	}
	return []byte(`"` + string(bs) + `"`), nil
}
