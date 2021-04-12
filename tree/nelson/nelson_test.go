package nelson_test

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/internal/testutils"
	"redwood.dev/state"
	"redwood.dev/tree/nelson"
)

type M = map[string]interface{}

func TestResolve_SimpleFrame(t *testing.T) {
	expectedVal := M{
		"blah": "hi",
		"asdf": []interface{}{uint64(5), uint64(4), uint64(3)},
	}

	db := testutils.SetupVersionedDBTreeWithValue(t, nil, M{
		"Content-Type":   "some content type",
		"Content-Length": int64(111),
		"value":          expectedVal,
	})
	defer db.DeleteDB()

	refResolver := &refResolverMock{}

	root := db.StateAtVersion(nil, false)
	defer root.Close()

	memroot, err := root.CopyToMemory(nil, nil)
	require.NoError(t, err)

	memroot, anyMissing, err := nelson.Resolve(memroot, refResolver)
	require.False(t, anyMissing)
	require.NoError(t, err)

	asNelSON, isNelSON := memroot.(*nelson.Frame)
	require.True(t, isNelSON)

	contentType, err := asNelSON.ContentType()
	require.NoError(t, err)
	require.Equal(t, "some content type", contentType)

	contentLength, err := asNelSON.ContentLength()
	require.NoError(t, err)
	require.Equal(t, int64(111), contentLength)

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(t, exists)
	require.NoError(t, err)
	require.Equal(t, expectedVal, val)
}

func TestResolve_SimpleFrameInFrame(t *testing.T) {
	db := testutils.SetupVersionedDBTreeWithValue(t, state.Keypath("foo"), M{
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
	})
	defer db.DeleteDB()

	refResolver := &refResolverMock{}

	root := db.StateAtVersion(nil, false)
	defer root.Close()

	memroot, err := root.CopyToMemory(nil, nil)
	require.NoError(t, err)

	memroot, anyMissing, err := nelson.Resolve(memroot, refResolver)
	require.False(t, anyMissing)
	require.NoError(t, err)

	_, isMemoryNode := memroot.(*state.MemoryNode)
	require.True(t, isMemoryNode)

	node := memroot.NodeAt(state.Keypath("foo/blah"), nil)
	asNelSON, isNelSON := node.(*nelson.Frame)
	require.True(t, isNelSON)

	contentType, err := asNelSON.ContentType()
	require.NoError(t, err)
	require.Equal(t, "inner", contentType)

	contentLength, err := asNelSON.ContentLength()
	require.NoError(t, err)
	require.Equal(t, int64(4321), contentLength)

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(t, exists)
	require.NoError(t, err)
	require.Equal(t, uint64(12345), val)
}

func TestResolve_SimpleFrameInFrameWithRef(t *testing.T) {
	db := testutils.SetupVersionedDBTreeWithValue(t, state.Keypath("foo"), M{
		"Merge-Type": M{
			"Content-Type": "resolver/js",
			"value": M{
				"src": M{
					"Content-Type": "link",
					"value":        "ref:sha3:deadbeefcafebabedeadbeefcafebabedeadbeefcafebabedeadbeefcafebabe",
				},
			},
		},
	})
	defer db.DeleteDB()

	stringReader := ioutil.NopCloser(strings.NewReader("xyzzy"))
	refResolver := &refResolverMock{
		refObjectReader: stringReader,
		refObjectLength: 5,
	}

	root := db.StateAtVersion(nil, false)
	defer root.Close()

	memroot, err := root.CopyToMemory(nil, nil)
	require.NoError(t, err)

	memroot, anyMissing, err := nelson.Resolve(memroot, refResolver)
	require.False(t, anyMissing)
	require.NoError(t, err)

	_, isMemoryNode := memroot.(*state.MemoryNode)
	require.True(t, isMemoryNode)

	node := memroot.NodeAt(state.Keypath("foo/Merge-Type"), nil)
	require.IsType(t, &nelson.Frame{}, node)

	asNelSON := node.(*nelson.Frame)

	contentType, err := asNelSON.ContentType()
	require.NoError(t, err)
	require.Equal(t, "resolver/js", contentType)

	contentLength, err := asNelSON.ContentLength()
	require.NoError(t, err)
	require.Equal(t, int64(0), contentLength)

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(t, exists)
	require.NoError(t, err)

	expected := M{
		"src": stringReader,
	}
	require.Equal(t, expected, val)

	// Now, test the innermost value, which should be the strings.Reader returned by the refResolverMock
	node = memroot.NodeAt(state.Keypath("foo/Merge-Type/src"), nil)
	require.IsType(t, &nelson.Frame{}, node)
	val, exists, err = node.Value(nil, nil)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, stringReader, val)

	contentLength, err = node.(nelson.ContentLengther).ContentLength()
	require.NoError(t, err)
	require.Equal(t, int64(5), contentLength)
}

func TestResolve_LinkToSimpleState(t *testing.T) {
	localDB := testutils.SetupVersionedDBTreeWithValue(t, state.Keypath("foo"), M{
		"blah": M{
			"Content-Type": "outer",
			"value": M{
				"Content-Length": float64(4321),
				"value": M{
					"Content-Type": "link",
					"value":        "state:otherState/someChannel/foo/xyzzy/zork",
				},
			},
		},
	})
	defer localDB.DeleteDB()

	otherDB := testutils.SetupVersionedDBTreeWithValue(t, state.Keypath("foo"), M{
		"xyzzy": M{
			"zork": uint64(55555),
		},
	})
	defer otherDB.DeleteDB()

	localState := localDB.StateAtVersion(nil, false)
	otherState := otherDB.StateAtVersion(nil, false)

	refResolver := &refResolverMock{
		stateURIs: map[string]state.Node{
			"otherState/someChannel": otherState,
		},
	}

	localStateMem, err := localState.CopyToMemory(nil, nil)
	require.NoError(t, err)

	localStateMem, anyMissing, err := nelson.Resolve(localStateMem, refResolver)
	require.False(t, anyMissing)
	require.NoError(t, err)

	asNelSON, isNelSON := localStateMem.NodeAt(state.Keypath("foo/blah"), nil).(*nelson.Frame)
	require.True(t, isNelSON)

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(t, exists)
	require.NoError(t, err)
	require.Equal(t, uint64(55555), val)
}

func TestResolve_LinkToMatryoshkaState(t *testing.T) {
	localDB := testutils.SetupVersionedDBTreeWithValue(t, state.Keypath("foo"), M{
		"blah": M{
			"Content-Type": "ignore this",
			"value": M{
				"Content-Length": int64(4321),
				"value": M{
					"Content-Type": "link",
					"value":        "state:otherState/someChannel/foo/xyzzy/zork",
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

	refResolver := &refResolverMock{
		stateURIs: map[string]state.Node{
			"otherState/someChannel": otherState,
		},
	}

	localStateMem, err := localState.CopyToMemory(nil, nil)
	require.NoError(t, err)

	localStateMem, anyMissing, err := nelson.Resolve(localStateMem, refResolver)
	require.False(t, anyMissing)
	require.NoError(t, err)

	asNelSON, isNelSON := localStateMem.NodeAt(state.Keypath("foo/blah"), nil).(*nelson.Frame)
	require.True(t, isNelSON)

	//require.Equal(t, "linked content type", asNelSON.ContentType())
	//require.Equal(t, int64(9999), asNelSON.ContentLength())

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(t, exists)
	require.NoError(t, err)
	require.Equal(t, uint64(55555), val)
}
