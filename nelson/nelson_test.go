package nelson

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strings"
	"testing"

	"github.com/brynbellomy/klog"
	"github.com/stretchr/testify/require"

	"github.com/brynbellomy/redwood/tree"
)

type M = map[string]interface{}

func setupDBTreeWithValue(T *testing.T, keypath tree.Keypath, val interface{}) *tree.DBTree {
	i := rand.Int()

	db, err := tree.NewDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(T, err)

	state := db.StateAtVersion(nil, true)
	defer state.Save()

	err = state.Set(keypath, nil, val)
	require.NoError(T, err)

	return db
}

func TestResolve_SimpleFrame(T *testing.T) {
	defer klog.Flush()
	flag.Set("logtostderr", "true")
	flag.Set("v", "2")

	expectedVal := M{
		"blah": "hi",
		"asdf": []interface{}{uint64(5), uint64(4), uint64(3)},
	}

	db := setupDBTreeWithValue(T, nil, M{
		"Content-Type":   "some content type",
		"Content-Length": int64(111),
		"value":          expectedVal,
	})
	defer db.DeleteDB()

	refResolver := &refResolverMock{}

	state := db.StateAtVersion(nil, false)
	defer state.Close()

	memstate, err := state.CopyToMemory(nil, nil)
	require.NoError(T, err)

	memstate, anyMissing, err := Resolve(memstate, refResolver)
	require.False(T, anyMissing)
	require.NoError(T, err)

	asNelSON, isNelSON := memstate.(*Frame)
	require.True(T, isNelSON)

	contentType, err := asNelSON.ContentType()
	require.NoError(T, err)
	require.Equal(T, "some content type", contentType)

	contentLength, err := asNelSON.ContentLength()
	require.NoError(T, err)
	require.Equal(T, int64(111), contentLength)

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, expectedVal, val)
}

func TestResolve_SimpleFrameInFrame(T *testing.T) {
	defer klog.Flush()
	flag.Set("logtostderr", "true")
	flag.Set("v", "2")

	db := setupDBTreeWithValue(T, tree.Keypath("foo"), M{
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

	state := db.StateAtVersion(nil, false)
	defer state.Close()

	memstate, err := state.CopyToMemory(nil, nil)
	require.NoError(T, err)

	memstate, anyMissing, err := Resolve(memstate, refResolver)
	require.False(T, anyMissing)
	require.NoError(T, err)

	_, isMemoryNode := memstate.(*tree.MemoryNode)
	require.True(T, isMemoryNode)

	node := memstate.NodeAt(tree.Keypath("foo/blah"), nil)
	asNelSON, isNelSON := node.(*Frame)
	require.True(T, isNelSON)

	contentType, err := asNelSON.ContentType()
	require.NoError(T, err)
	require.Equal(T, "inner", contentType)

	contentLength, err := asNelSON.ContentLength()
	require.NoError(T, err)
	require.Equal(T, int64(4321), contentLength)

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, uint64(12345), val)
}

func TestResolve_SimpleFrameInFrameWithRef(T *testing.T) {
	defer klog.Flush()
	flag.Set("logtostderr", "true")
	flag.Set("v", "2")

	db := setupDBTreeWithValue(T, tree.Keypath("foo"), M{
		"Merge-Type": M{
			"Content-Type": "resolver/js",
			"value": M{
				"src": M{
					"Content-Type": "link",
					"value":        "ref:deadbeef",
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

	state := db.StateAtVersion(nil, false)
	defer state.Close()

	memstate, err := state.CopyToMemory(nil, nil)
	require.NoError(T, err)

	memstate, anyMissing, err := Resolve(memstate, refResolver)
	require.False(T, anyMissing)
	require.NoError(T, err)

	_, isMemoryNode := memstate.(*tree.MemoryNode)
	require.True(T, isMemoryNode)

	node := memstate.NodeAt(tree.Keypath("foo/Merge-Type"), nil)
	require.IsType(T, &Frame{}, node)

	asNelSON := node.(*Frame)

	contentType, err := asNelSON.ContentType()
	require.NoError(T, err)
	require.Equal(T, "resolver/js", contentType)

	contentLength, err := asNelSON.ContentLength()
	require.NoError(T, err)
	require.Equal(T, int64(0), contentLength)

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(T, exists)
	require.NoError(T, err)

	expected := M{
		"src": stringReader,
	}
	require.Equal(T, expected, val)

	// Now, test the innermost value, which should be the strings.Reader returned by the refResolverMock
	node = memstate.NodeAt(tree.Keypath("foo/Merge-Type/src"), nil)
	require.IsType(T, &Frame{}, node)
	val, exists, err = node.Value(nil, nil)
	require.NoError(T, err)
	require.True(T, exists)
	require.Equal(T, stringReader, val)

	contentLength, err = node.ContentLength()
	require.NoError(T, err)
	require.Equal(T, int64(5), contentLength)
}

func TestResolve_LinkToSimpleState(T *testing.T) {
	defer klog.Flush()
	flag.Set("logtostderr", "true")
	flag.Set("v", "2")

	localDB := setupDBTreeWithValue(T, tree.Keypath("foo"), M{
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

	otherDB := setupDBTreeWithValue(T, tree.Keypath("foo"), M{
		"xyzzy": M{
			"zork": uint64(55555),
		},
	})
	defer otherDB.DeleteDB()

	localState := localDB.StateAtVersion(nil, false)
	otherState := otherDB.StateAtVersion(nil, false)

	refResolver := &refResolverMock{
		stateURIs: map[string]tree.Node{
			"otherState/someChannel": otherState,
		},
	}

	localStateMem, err := localState.CopyToMemory(nil, nil)
	require.NoError(T, err)

	localStateMem, anyMissing, err := Resolve(localStateMem, refResolver)
	require.False(T, anyMissing)
	require.NoError(T, err)

	asNelSON, isNelSON := localStateMem.NodeAt(tree.Keypath("foo/blah"), nil).(*Frame)
	require.True(T, isNelSON)

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, uint64(55555), val)
}

func TestResolve_LinkToMatryoshkaState(T *testing.T) {
	defer klog.Flush()
	flag.Set("logtostderr", "true")
	flag.Set("v", "2")

	localDB := setupDBTreeWithValue(T, tree.Keypath("foo"), M{
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

	otherDB := setupDBTreeWithValue(T, tree.Keypath("foo"), M{
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
		stateURIs: map[string]tree.Node{
			"otherState/someChannel": otherState,
		},
	}

	localStateMem, err := localState.CopyToMemory(nil, nil)
	require.NoError(T, err)

	localStateMem, anyMissing, err := Resolve(localStateMem, refResolver)
	require.False(T, anyMissing)
	require.NoError(T, err)

	asNelSON, isNelSON := localStateMem.NodeAt(tree.Keypath("foo/blah"), nil).(*Frame)
	require.True(T, isNelSON)

	//require.Equal(T, "linked content type", asNelSON.ContentType())
	//require.Equal(T, int64(9999), asNelSON.ContentLength())

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, uint64(55555), val)
}
