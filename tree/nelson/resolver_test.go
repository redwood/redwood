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
	"redwood.dev/types"
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
			frame, remainingKeypath, err := resolver.HelperCollapseBasicFrame(node, test.keypath)
			assert.Equal(t, test.expectedErr, errors.Cause(err))
			assert.Len(t, remainingKeypath, 0)
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

	resolvedNode.DebugPrint(utils.PrintfDebugPrinter, true, 0)

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

	otherDB := testutils.SetupVersionedDBTreeWithValue(t, nil, M{
		"foo": M{
			"xyzzy": M{
				"zork": M{
					"Content-Type":   "linked content type",
					"Content-Length": int64(9999),
					"value":          uint64(55555),
				},
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

	// require.Equal(t, "linked content type", asNelSON.ContentType())

	val, exists, err := asNelSON.Value(nil, nil)
	require.True(t, exists)
	require.NoError(t, err)
	require.Equal(t, uint64(55555), val)
}

func TestResolver_TextEditorDemo(t *testing.T) {
	tree := M{
		"text": M{
			"value": "",
			"Merge-Type": M{
				"Content-Type": "resolver/js",
				"value": M{
					"src": M{
						"Content-Type": "link",
						"value":        "blob:sha3:deadbeefcafebabedeadbeefcafebabedeadbeefcafebabedeadbeefcafebabe",
					},
					"xyzzy": 123,
				},
			},
		},
	}

	resolverConfigKeypath := state.Keypath("text/Merge-Type")

	var blobID blob.ID
	err := blobID.UnmarshalText([]byte("sha3:deadbeefcafebabedeadbeefcafebabedeadbeefcafebabedeadbeefcafebabe"))
	require.NoError(t, err)

	blobResolver := &blobResolverMock{}
	blobResolver.addBlobResource(blobID, ioutil.NopCloser(bytes.NewReader([]byte("xyzzy"))), blob.Manifest{TotalSize: 5}, nil)

	resolver := nelson.NewResolver(&stateResolverMock{}, blobResolver, nil)

	db := testutils.SetupDBTreeWithValue(t, nil, tree)
	defer db.DeleteDB()

	node := db.State(false)
	defer node.Close()

	config, err := node.CopyToMemory(resolverConfigKeypath, nil)
	require.NoError(t, err)

	resolved, anyMissing, err := resolver.Resolve(config)
	require.NoError(t, err)
	require.False(t, anyMissing)

	require.IsType(t, nelson.Frame{}, resolved)
	require.Equal(t, "resolver/js", resolved.(nelson.Frame).ContentType())

	srcNode := resolved.NodeAt(state.Keypath("src"), nil)
	require.IsType(t, nelson.BlobFrame{}, srcNode)

	reader, length, err := srcNode.(nelson.Node).BytesReader(nil)
	require.NoError(t, err)
	require.Equal(t, int64(5), length)

	bs, err := ioutil.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, []byte("xyzzy"), bs)
}

func TestResolver_GitIntegrationDemo(t *testing.T) {
	var (
		readmeSHA3 = testutils.RandomHash(t)
		indexSHA3  = testutils.RandomHash(t)
		jpegSHA3   = testutils.RandomHash(t)
		jsSHA3     = testutils.RandomHash(t)

		readmeContents = "this is the readme"
		indexContents  = "this is the index"
		jpegContents   = "this is the jpeg"
		jsContents     = "this is the js"
	)

	tree := M{
		"Merge-Type": M{
			"Content-Type": "resolver/dumb",
			"value":        M{},
		},
		"Validator": M{
			"Content-Type": "validator/permissions",
			"value": M{
				"*": M{
					"^.*": M{
						"write": true,
					},
				},
				"269cb7b224e06c3cdaecdb0088d9613f64418a84": M{
					"^.*$": M{
						"write": true,
					},
				},
			},
		},
		"commits": M{
			"305c6e97a68ec38b4b89a4f1e5bc18c8ba2b76eb": M{
				"author": M{
					"email":     "foo@bar.com",
					"name":      "Foo Bar",
					"timestamp": "2022-01-10T19:44:38-06:00",
				},
				"committer": M{
					"email":     "foo@bar.com",
					"name":      "Foo Bar",
					"timestamp": "2022-01-10T19:44:38-06:00",
				},
				"files": M{
					"README.md": M{
						"Content-Length": 104,
						"Content-Type":   "link",
						"mode":           33188,
						"sha1":           "85508cd2bf9c6142b314faf1f9a60edb1f2d376b000000000000000000000000",
						"value":          "blob:sha3:" + readmeSHA3.Hex(),
					},
					"index.html": M{
						"Content-Length": 8493,
						"Content-Type":   "link",
						"mode":           33188,
						"sha1":           "712e4bd59f86940a02996a882d4b891870c35abd000000000000000000000000",
						"value":          "blob:sha3:" + indexSHA3.Hex(),
					},
					"redwood.jpg": M{
						"Content-Length": 1198230,
						"Content-Type":   "link",
						"mode":           33188,
						"sha1":           "f624d0963ebcf778a724c67cb745ddfb85997469000000000000000000000000",
						"value":          "blob:sha3:" + jpegSHA3.Hex(),
					},
					"script.js": M{
						"Content-Length": 62,
						"Content-Type":   "link",
						"mode":           33188,
						"sha1":           "992fb54c32d96c7f84afccd9fa6a62d59ecce012000000000000000000000000",
						"value":          "blob:sha3:" + jsSHA3.Hex(),
					},
				},
				"message":   "First commit\n",
				"sig":       "",
				"timestamp": "2022-01-10T19:44:38-06:00",
			},
		},
		"demo": M{
			"Content-Type": "link",
			"value":        "state:somegitprovider.org/gitdemo/refs/heads/master/worktree",
		},
		"refs": M{
			"heads": M{
				"master": M{
					"HEAD": "305c6e97a68ec38b4b89a4f1e5bc18c8ba2b76eb",
					"worktree": M{
						"Content-Type": "link",
						"value":        "state:somegitprovider.org/gitdemo/commits/305c6e97a68ec38b4b89a4f1e5bc18c8ba2b76eb/files",
					},
				},
			},
		},
	}

	db := testutils.SetupDBTreeWithValue(t, nil, tree)
	defer db.DeleteDB()

	node := db.State(false)
	defer node.Close()

	blobResolver := &blobResolverMock{}
	blobResolver.addBlobResource(blob.ID{types.SHA3, readmeSHA3}, testutils.ReadCloserFromString(readmeContents), testutils.MakeBlobManifest(uint64(len(readmeContents))), nil)
	blobResolver.addBlobResource(blob.ID{types.SHA3, indexSHA3}, testutils.ReadCloserFromString(indexContents), testutils.MakeBlobManifest(uint64(len(indexContents))), nil)
	blobResolver.addBlobResource(blob.ID{types.SHA3, jpegSHA3}, testutils.ReadCloserFromString(jpegContents), testutils.MakeBlobManifest(uint64(len(jpegContents))), nil)
	blobResolver.addBlobResource(blob.ID{types.SHA3, jsSHA3}, testutils.ReadCloserFromString(jsContents), testutils.MakeBlobManifest(uint64(len(jsContents))), nil)

	stateResolver := &stateResolverMock{}
	stateResolver.addStateResource("somegitprovider.org/gitdemo", node, nil)

	resolver := nelson.NewResolver(stateResolver, blobResolver, nil)

	// copied, err := node.CopyToMemory(nil, nil)
	soughtNode, exists, err := resolver.Seek(node, state.Keypath("demo/README.md"))
	require.NoError(t, err)
	require.True(t, exists)

	// resolved, anyMissing, err := resolver.Resolve(copied)
	// require.NoError(t, err)
	// require.False(t, anyMissing)

	soughtNode.DebugPrint(utils.PrintfDebugPrinter, true, 0)

}
