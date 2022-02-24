package nelson_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/internal/testutils"
	"redwood.dev/state"
	"redwood.dev/tree/nelson"
	"redwood.dev/utils"
)

type M = map[string]interface{}

func mustJSON(t *testing.T, x interface{}) []byte {
	bs, err := json.Marshal(x)
	require.NoError(t, err)
	return bs
}

func TestResolver_DrillDownUntilFrame(t *testing.T) {
	val := M{
		"foo": M{
			"blah": M{
				"Content-Type":   "outer",
				"Content-Length": int64(111),
				"value": M{
					"Content-Length": int64(4321),
					"value": M{
						"Content-Type": "inner",
						"value":        uint64(12345),
					},
				},
			},
		},
	}

	tests := []struct {
		name                     string
		keypath                  state.Keypath
		expectedFrame            M
		expectedNonFrame         M
		expectedRemainingKeypath state.Keypath
		expectedErr              error
	}{
		{"", state.Keypath("foo"), nil, val["foo"].(M), nil, nil},
		{"", state.Keypath("foo/blah"), val["foo"].(M)["blah"].(M), nil, nil, nil},
		{"", state.Keypath("foo/blah/value"), val["foo"].(M)["blah"].(M), nil, state.Keypath("value"), nil},
		{"", state.Keypath("foo/blah/yeet"), val["foo"].(M)["blah"].(M), nil, state.Keypath("yeet"), nil},
		{"", state.Keypath("foo/yeet"), nil, nil, nil, errors.Err404},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			db := testutils.SetupVersionedDBTreeWithValue(t, nil, val)
			defer db.DeleteDB()
			node := db.StateAtVersion(nil, false)
			defer node.Close()

			resolver := nelson.NewResolver(nil, nil, nil)
			frameNode, nonFrameNode, remainingKeypath, err := resolver.HelperDrillDownUntilFrame(node, test.keypath)
			assert.Equal(t, test.expectedErr, errors.Cause(err))
			assert.Equal(t, mustJSON(t, test.expectedFrame), mustJSON(t, frameNode))
			assert.Equal(t, mustJSON(t, test.expectedNonFrame), mustJSON(t, nonFrameNode))
			assert.Equal(t, test.expectedRemainingKeypath, remainingKeypath)
		})
	}
}

func TestResolver_CollapseBasicFrame(t *testing.T) {
	val := M{
		"Content-Type":   "outer",
		"Content-Length": int64(111),
		"value": M{
			"Content-Length": int64(4321),
			"value": M{
				"Content-Type": "inner",
				"value":        uint64(12345),
			},
		},
	}

	tests := []struct {
		name          string
		keypath       state.Keypath
		expectedFrame interface{}
		expectedErr   error
	}{
		{"", nil, val["value"].(M)["value"].(M)["value"], nil},
		// {"", state.Keypath("foo"), nil, val["foo"].(M), nil, nil},
		// {"", state.Keypath("foo/blah"), val["foo"].(M)["blah"].(M), nil, nil, nil},
		// {"", state.Keypath("foo/blah/value"), val["foo"].(M)["blah"].(M), nil, state.Keypath("value"), nil},
		// {"", state.Keypath("foo/blah/yeet"), val["foo"].(M)["blah"].(M), nil, state.Keypath("yeet"), nil},
		// {"", state.Keypath("foo/yeet"), nil, nil, nil, errors.Err404},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			db := testutils.SetupVersionedDBTreeWithValue(t, nil, val)
			defer db.DeleteDB()
			node := db.StateAtVersion(nil, false)
			defer node.Close()

			resolver := nelson.NewResolver(nil, nil, nil)
			frame, err := resolver.HelperCollapseBasicFrame(node, test.keypath)
			assert.Equal(t, test.expectedErr, errors.Cause(err))
			assert.Equal(t, mustJSON(t, test.expectedFrame), mustJSON(t, frame))
		})
	}
}

func TestResolver_SimpleFrame(t *testing.T) {
	expectedVal := M{
		"blah": "hi",
		"asdf": []interface{}{uint64(5), uint64(4), uint64(3)},
	}

	db := testutils.SetupVersionedDBTreeWithValue(t, nil, M{
		"Content-Type": "some content type",
		"value":        expectedVal,
	})
	defer db.DeleteDB()

	resolver := nelson.NewResolver(nil, nil, nil)

	root := db.StateAtVersion(nil, false)
	defer root.Close()

	memroot, err := root.CopyToMemory(nil, nil)
	require.NoError(t, err)

	resolvedNode, anyMissing, err := resolver.Resolve(memroot)
	require.False(t, anyMissing)
	require.NoError(t, err)

	require.IsType(t, nelson.Frame{}, resolvedNode)
	asNelSON := resolvedNode.(nelson.Frame)

	contentType := asNelSON.ContentType()
	require.Equal(t, "some content type", contentType)

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(t, exists)
	require.NoError(t, err)
	require.Equal(t, expectedVal, val)
}

func TestResolver_SimpleFrameInFrame(t *testing.T) {
	db := testutils.SetupVersionedDBTreeWithValue(t, nil, M{
		"foo": M{
			"blah": M{
				"Content-Type":   "outer",
				"Content-Length": int64(111),
				"value": M{
					"Content-Length": int64(4321),
					"value": M{
						"Content-Type": "inner",
						"value":        uint64(12345),
					},
				},
			},
		},
	})
	defer db.DeleteDB()

	root := db.StateAtVersion(nil, false)
	defer root.Close()

	memroot, err := root.CopyToMemory(nil, nil)
	require.NoError(t, err)

	resolver := nelson.NewResolver(nil, nil, nil)

	resolvedNode, anyMissing, err := resolver.Resolve(memroot)
	require.NoError(t, err)
	require.False(t, anyMissing)

	require.IsType(t, &state.MemoryNode{}, resolvedNode)

	node := resolvedNode.NodeAt(state.Keypath("foo/blah"), nil)
	asNelSON, isNelSON := node.(nelson.Frame)
	require.True(t, isNelSON)

	contentType := asNelSON.ContentType()
	require.Equal(t, "inner", contentType)

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(t, exists)
	require.NoError(t, err)
	require.Equal(t, uint64(12345), val)
}

func TestResolver_SimpleFrameInFrameWithBlob(t *testing.T) {
	db := testutils.SetupVersionedDBTreeWithValue(t, state.Keypath("foo"), M{
		"Merge-Type": M{
			"Content-Type": "resolver/js",
			"value": M{
				"src": M{
					"Content-Type": "link",
					"value":        "blob:sha3:deadbeefcafebabedeadbeefcafebabedeadbeefcafebabedeadbeefcafebabe",
				},
			},
		},
	})
	defer db.DeleteDB()

	var blobID blob.ID
	err := blobID.UnmarshalText([]byte("sha3:deadbeefcafebabedeadbeefcafebabedeadbeefcafebabedeadbeefcafebabe"))
	require.NoError(t, err)

	blobResolver := &blobResolverMock{}
	blobResolver.addBlobResource(blobID, ioutil.NopCloser(bytes.NewReader([]byte("xyzzy"))), blob.Manifest{TotalSize: 5}, nil)
	resolver := nelson.NewResolver(nil, blobResolver, nil)

	root := db.StateAtVersion(nil, false)
	defer root.Close()

	memroot, err := root.CopyToMemory(nil, nil)
	require.NoError(t, err)

	memroot, anyMissing, err := resolver.Resolve(memroot)
	require.False(t, anyMissing)
	require.NoError(t, err)

	_, isMemoryNode := memroot.(*state.MemoryNode)
	require.True(t, isMemoryNode)

	node := memroot.NodeAt(state.Keypath("foo/Merge-Type"), nil)
	require.IsType(t, nelson.Frame{}, node)
	asNelSON := node.(nelson.Frame)

	contentType := asNelSON.ContentType()
	require.Equal(t, "resolver/js", contentType)

	maybeBlobFrame := asNelSON.NodeAt(state.Keypath("src"), nil)
	require.IsType(t, nelson.BlobFrame{}, maybeBlobFrame)
	blobFrame := maybeBlobFrame.(nelson.BlobFrame)

	reader, length, err := blobFrame.BytesReader(nil)
	require.NoError(t, err)
	require.Equal(t, int64(5), length)

	bs, err := ioutil.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "xyzzy", string(bs))
}

func TestResolver_LinkToSimpleState(t *testing.T) {
	localDB := testutils.SetupVersionedDBTreeWithValue(t, nil, M{
		"foo": M{
			"blah": M{
				"Content-Type": "outer",
				"value": M{
					"Content-Length": float64(4321),
					"value": M{
						"Content-Type": "link",
						"value":        "state:otherState.foo/someChannel/bar/xyzzy/zork",
					},
				},
			},
		},
	})
	defer localDB.DeleteDB()

	otherDB := testutils.SetupVersionedDBTreeWithValue(t, nil, M{
		"bar": M{
			"xyzzy": M{
				"zork": uint64(55555),
			},
		},
	})
	defer otherDB.DeleteDB()

	localState := localDB.StateAtVersion(nil, false)
	otherState := otherDB.StateAtVersion(nil, false)

	stateResolver := &stateResolverMock{}
	stateResolver.addStateResource("otherState.foo/someChannel", otherState, nil)
	resolver := nelson.NewResolver(stateResolver, nil, nil)

	localStateMem, err := localState.CopyToMemory(nil, nil)
	require.NoError(t, err)

	localStateMem, anyMissing, err := resolver.Resolve(localStateMem)
	require.False(t, anyMissing)
	require.NoError(t, err)

	localStateMem.DebugPrint(utils.PrintfDebugPrinter, true, 0)

	maybeFrame := localStateMem.NodeAt(state.Keypath("foo/blah"), nil)
	require.IsType(t, nelson.Frame{}, maybeFrame)
	asNelSON := maybeFrame.(nelson.Frame)

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(t, exists)
	require.NoError(t, err)
	require.Equal(t, uint64(55555), val)
}

func TestResolver_LinkToMatryoshkaState(t *testing.T) {
	localDB := testutils.SetupVersionedDBTreeWithValue(t, state.Keypath("foo"), M{
		"blah": M{
			"Content-Type": "ignore this",
			"value": M{
				"Content-Length": int64(4321),
				"value": M{
					"Content-Type": "link",
					"value":        "state:otherState.foo/someChannel/foo/xyzzy/zork",
				},
			},
		},
	})
	defer localDB.DeleteDB()

	otherDB := testutils.SetupVersionedDBTreeWithValue(t, state.Keypath("foo"), M{
		"xyzzy": M{
			"zork": M{
				"Content-Type":   "linked content type",
				"Content-Length": int64(9999),
				"value":          uint64(55555),
			},
		},
	})
	defer otherDB.DeleteDB()

	localState := localDB.StateAtVersion(nil, false)
	otherState := otherDB.StateAtVersion(nil, false)

	stateResolver := &stateResolverMock{}
	stateResolver.addStateResource("otherState.foo/someChannel", otherState, nil)
	resolver := nelson.NewResolver(stateResolver, nil, nil)

	localStateMem, err := localState.CopyToMemory(nil, nil)
	require.NoError(t, err)

	localStateMem, anyMissing, err := resolver.Resolve(localStateMem)
	require.False(t, anyMissing)
	require.NoError(t, err)

	soughtNode := localStateMem.NodeAt(state.Keypath("foo/blah"), nil)
	require.IsType(t, nelson.Frame{}, soughtNode)
	asNelSON := soughtNode.(nelson.Frame)

	require.Equal(t, "linked content type", asNelSON.ContentType())

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(t, exists)
	require.NoError(t, err)
	require.Equal(t, uint64(55555), val)
}
