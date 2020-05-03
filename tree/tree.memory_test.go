package tree

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemoryNode_Set(T *testing.T) {
	tests := []struct {
		name       string
		setKeypath Keypath
		atKeypath  Keypath
		rng        *Range
		fixture    fixture
	}{
		{"root keypath, single set, map value", Keypath(nil), Keypath(nil), nil, fixture1},
		{"root keypath, single set, map value 2", Keypath(nil), Keypath(nil), nil, fixture2},
		{"root keypath, single set, float value", Keypath(nil), Keypath(nil), nil, fixture5},
		{"root keypath, single set, string value", Keypath(nil), Keypath(nil), nil, fixture6},
		{"root keypath, single set, bool value", Keypath(nil), Keypath(nil), nil, fixture7},

		{"non-root keypath, single set, map value", Keypath("foo/bar"), Keypath(nil), nil, fixture1},
		{"non-root keypath, single set, map value 2", Keypath("foo/bar"), Keypath(nil), nil, fixture2},
		{"non-root keypath, single set, float value", Keypath("foo/bar"), Keypath(nil), nil, fixture5},
		{"non-root keypath, single set, string value", Keypath("foo/bar"), Keypath(nil), nil, fixture6},
		{"non-root keypath, single set, bool value", Keypath("foo/bar"), Keypath(nil), nil, fixture7},
	}

	for _, test := range tests {
		test := test
		T.Run(test.name, func(T *testing.T) {
			state := NewMemoryNode().NodeAt(test.atKeypath, nil).(*MemoryNode)

			err := state.Set(test.setKeypath, test.rng, test.fixture.input)
			require.NoError(T, err)

			prefixOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			expectedNodesFromSetKeypath := len(prefixOutputs)

			expectedNumValues := countNodesOfType(NodeTypeValue, test.fixture.output)
			expectedNumSlices := countNodesOfType(NodeTypeSlice, test.fixture.output)
			expectedNumMaps := countNodesOfType(NodeTypeMap, test.fixture.output) + expectedNodesFromSetKeypath
			expectedNumNodes := expectedNumValues + expectedNumSlices + expectedNumMaps
			require.Equal(T, expectedNumValues, len(state.values))
			require.Equal(T, expectedNumNodes, len(state.nodeTypes))
			require.Equal(T, expectedNumNodes, len(state.keypaths))

			expected := prefixFixtureOutputs(test.setKeypath, test.fixture.output)
			expected = append(prefixOutputs, expected...)

			for i, kp := range state.keypaths {
				require.Equal(T, expected[i].keypath, kp)
				require.Equal(T, expected[i].nodeType, state.nodeTypes[string(kp)])
				if state.nodeTypes[string(kp)] == NodeTypeValue {
					require.Equal(T, expected[i].value, state.values[string(kp)])
				}
			}
		})
	}
}

func TestMemoryNode_SetNode(T *testing.T) {
	T.Run("inner MemoryNode", func(T *testing.T) {
		state := NewMemoryNode().(*MemoryNode)
		innerNode := NewMemoryNode().(*MemoryNode)

		err := state.Set(Keypath("foo"), nil, M{
			"one":   uint64(123),
			"two":   M{"two a": false, "two b": nil},
			"three": "hello",
		})
		require.NoError(T, err)

		err = innerNode.Set(Keypath("bar"), nil, M{
			"baz": M{
				"xyzzy": "zork",
			},
		})
		require.NoError(T, err)

		err = state.Set(Keypath("foo/two"), nil, innerNode)
		require.NoError(T, err)

		require.Len(T, state.keypaths, 5)

		expected := M{
			"foo": M{
				"one": uint64(123),
				"two": M{
					"bar": M{
						"baz": M{
							"xyzzy": "zork",
						},
					},
				},
				"three": "hello",
			},
		}

		received, exists, err := state.Value(nil, nil)
		require.NoError(T, err)
		require.True(T, exists)
		require.Equal(T, expected, received)
	})
}

func TestMemoryNode_ParentNodeFor(T *testing.T) {
	T.Run("non-root node", func(T *testing.T) {
		state := NewMemoryNode().(*MemoryNode)
		innerNode := NewMemoryNode().(*MemoryNode)

		err := state.Set(Keypath("foo"), nil, M{
			"one":   uint64(123),
			"two":   M{"two a": false, "two b": nil},
			"three": "hello",
		})
		require.NoError(T, err)

		err = innerNode.Set(Keypath("bar"), nil, M{
			"baz": M{
				"xyzzy": "zork",
			},
		})
		require.NoError(T, err)

		err = state.Set(Keypath("foo/two"), nil, innerNode)
		require.NoError(T, err)

		node, keypath := state.ParentNodeFor(Keypath("foo/two/bar/baz/xyzzy"))
		require.Equal(T, Keypath("bar/baz/xyzzy"), keypath)
		memNode := node.(*MemoryNode)
		require.Equal(T, innerNode.keypaths, memNode.keypaths)
	})

	T.Run("root node", func(T *testing.T) {
		state := NewMemoryNode().(*MemoryNode)
		innerNode := NewMemoryNode().(*MemoryNode)

		err := state.Set(Keypath("foo"), nil, M{
			"one":   uint64(123),
			"two":   M{"two a": false, "two b": nil},
			"three": "hello",
		})
		require.NoError(T, err)
		state.DebugPrint()

		err = innerNode.Set(Keypath("bar"), nil, M{
			"baz": M{
				"xyzzy": "zork",
			},
		})
		require.NoError(T, err)
		innerNode.DebugPrint()

		err = state.Set(nil, nil, innerNode)
		require.NoError(T, err)
		state.DebugPrint()

		node, keypath := state.ParentNodeFor(Keypath("bar/baz/xyzzy"))
		require.Equal(T, Keypath("bar/baz/xyzzy"), keypath)
		memNode := node.(*MemoryNode)
		require.Equal(T, innerNode.keypaths, memNode.keypaths)
	})
}

func TestMemoryNode_Delete(T *testing.T) {
	tests := []struct {
		name          string
		setKeypath    Keypath
		setRange      *Range
		deleteKeypath Keypath
		deleteRange   *Range
		fixture       fixture
	}{
		{"root keypath, single set, map value", Keypath(nil), nil, Keypath("flox"), nil, fixture1},
		{"root keypath, single set, map value 2", Keypath(nil), nil, Keypath("eee"), nil, fixture2},
		{"root keypath, single set, float value", Keypath(nil), nil, Keypath(nil), nil, fixture5},
		{"root keypath, single set, string value", Keypath(nil), nil, Keypath(nil), nil, fixture6},
		{"root keypath, single set, bool value", Keypath(nil), nil, Keypath(nil), nil, fixture7},

		{"non-root keypath, single set, map value", Keypath("foo/bar"), nil, Keypath(nil), nil, fixture1},
		{"non-root keypath, single set, map value 2", Keypath("foo/bar"), nil, Keypath(nil), nil, fixture2},
		{"non-root keypath, single set, float value", Keypath("foo/bar"), nil, Keypath(nil), nil, fixture5},
		{"non-root keypath, single set, string value", Keypath("foo/bar"), nil, Keypath(nil), nil, fixture6},
		{"non-root keypath, single set, bool value", Keypath("foo/bar"), nil, Keypath(nil), nil, fixture7},
	}

	for _, test := range tests {
		test := test
		T.Run(test.name, func(T *testing.T) {
			state := NewMemoryNode().(*MemoryNode)

			err := state.Set(test.setKeypath, test.setRange, test.fixture.input)
			require.NoError(T, err)

			err = state.Delete(test.deleteKeypath, test.deleteRange)
			require.NoError(T, err)

			remainingOutputs := removeFixtureOutputsWithPrefix(test.deleteKeypath, test.fixture.output...)

			// expectedNumValues := countNodesOfType(NodeTypeValue, remainingOutputs)
			// expectedNumSlices := countNodesOfType(NodeTypeSlice, remainingOutputs)
			// expectedNumMaps := countNodesOfType(NodeTypeMap, remainingOutputs) + test.deleteKeypath.NumParts()
			// expectedNumNodes := expectedNumValues + expectedNumSlices + expectedNumMaps
			// require.Equal(T, expectedNumValues, len(state.values))
			// require.Equal(T, expectedNumNodes, len(state.nodeTypes))
			// require.Equal(T, expectedNumNodes, len(state.keypaths))

			setKeypathOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			expected := append(setKeypathOutputs, remainingOutputs...)
			for i, kp := range state.keypaths {
				require.Equal(T, expected[i].keypath, kp)
				require.Equal(T, expected[i].nodeType, state.nodeTypes[string(kp)])
				if state.nodeTypes[string(kp)] == NodeTypeValue {
					require.Equal(T, expected[i].value, state.values[string(kp)])
				}
			}
		})
	}
}

func TestMemoryNode_Iterator(T *testing.T) {
	tests := []struct {
		name        string
		setKeypath  Keypath
		iterKeypath Keypath
		fixture     fixture
	}{
		{"root set, root iter, map value", Keypath(nil), Keypath(nil), fixture1},
		{"root set, root iter, map value 2", Keypath(nil), Keypath(nil), fixture2},
		{"root set, root iter, float value", Keypath(nil), Keypath(nil), fixture5},
		{"root set, root iter, string value", Keypath(nil), Keypath(nil), fixture6},
		{"root set, root iter, bool value", Keypath(nil), Keypath(nil), fixture7},

		{"non-root set, root iter, map value", Keypath("foo/bar"), Keypath(nil), fixture1},
		{"non-root set, root iter, map value 2", Keypath("foo/bar"), Keypath(nil), fixture2},
		{"non-root set, root iter, float value", Keypath("foo/bar"), Keypath(nil), fixture5},
		{"non-root set, root iter, string value", Keypath("foo/bar"), Keypath(nil), fixture6},
		{"non-root set, root iter, bool value", Keypath("foo/bar"), Keypath(nil), fixture7},

		{"root set, non-root iter, map value", Keypath(nil), Keypath("flox"), fixture1},
		{"root set, non-root iter, map value 2", Keypath(nil), Keypath("flox"), fixture2},
		{"root set, non-root iter, float value", Keypath(nil), Keypath("flox"), fixture5},
		{"root set, non-root iter, string value", Keypath(nil), Keypath("flox"), fixture6},
		{"root set, non-root iter, bool value", Keypath(nil), Keypath("flox"), fixture7},

		{"non-root set, non-root iter, map value", Keypath("foo/bar"), Keypath("flox"), fixture1},
		{"non-root set, non-root iter, map value 2", Keypath("foo/bar"), Keypath("flox"), fixture2},
		{"non-root set, non-root iter, float value", Keypath("foo/bar"), Keypath("flox"), fixture5},
		{"non-root set, non-root iter, string value", Keypath("foo/bar"), Keypath("flox"), fixture6},
		{"non-root set, non-root iter, bool value", Keypath("foo/bar"), Keypath("flox"), fixture7},
	}

	for _, test := range tests {
		test := test
		T.Run(test.name, func(T *testing.T) {
			state := NewMemoryNode()
			state.Set(test.setKeypath, nil, test.fixture.input)

			setKeypathOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			valueOutputs := prefixFixtureOutputs(test.setKeypath, test.fixture.output)
			expected := append(setKeypathOutputs, valueOutputs...)
			expected = filterFixtureOutputsWithPrefix(test.iterKeypath, expected...)

			iter := state.Iterator(test.iterKeypath, false, 0)
			defer iter.Close()
			var i int
			for iter.Rewind(); iter.Valid(); iter.Next() {
				node := iter.Node()
				require.Equal(T, expected[i].keypath, node.Keypath())
				i++
			}
			require.Equal(T, len(expected), i)

		})
	}
}

func TestMemoryNode_ChildIterator(T *testing.T) {
	tests := []struct {
		name        string
		setKeypath  Keypath
		iterKeypath Keypath
		fixture     fixture
	}{
		{"root set, root iter, map value", Keypath(nil), Keypath(nil), fixture1},
		{"root set, root iter, map value 2", Keypath(nil), Keypath(nil), fixture2},
		{"root set, root iter, float value", Keypath(nil), Keypath(nil), fixture5},
		{"root set, root iter, string value", Keypath(nil), Keypath(nil), fixture6},
		{"root set, root iter, bool value", Keypath(nil), Keypath(nil), fixture7},

		{"non-root set, root iter, map value", Keypath("foo/bar"), Keypath(nil), fixture1},
		{"non-root set, root iter, map value 2", Keypath("foo/bar"), Keypath(nil), fixture2},
		{"non-root set, root iter, float value", Keypath("foo/bar"), Keypath(nil), fixture5},
		{"non-root set, root iter, string value", Keypath("foo/bar"), Keypath(nil), fixture6},
		{"non-root set, root iter, bool value", Keypath("foo/bar"), Keypath(nil), fixture7},

		{"root set, non-root iter, map value", Keypath(nil), Keypath("flox"), fixture1},
		{"root set, non-root iter, map value 2", Keypath(nil), Keypath("flox"), fixture2},
		{"root set, non-root iter, float value", Keypath(nil), Keypath("flox"), fixture5},
		{"root set, non-root iter, string value", Keypath(nil), Keypath("flox"), fixture6},
		{"root set, non-root iter, bool value", Keypath(nil), Keypath("flox"), fixture7},

		{"non-root set, non-root iter, map value", Keypath("foo/bar"), Keypath("flox"), fixture1},
		{"non-root set, non-root iter, map value 2", Keypath("foo/bar"), Keypath("flox"), fixture2},
		{"non-root set, non-root iter, float value", Keypath("foo/bar"), Keypath("flox"), fixture5},
		{"non-root set, non-root iter, string value", Keypath("foo/bar"), Keypath("flox"), fixture6},
		{"non-root set, non-root iter, bool value", Keypath("foo/bar"), Keypath("flox"), fixture7},
	}

	for _, test := range tests {
		test := test
		T.Run(test.name, func(T *testing.T) {
			state := NewMemoryNode()
			state.Set(test.setKeypath, nil, test.fixture.input)

			setKeypathOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			prefixedOutputs := prefixFixtureOutputs(test.setKeypath, test.fixture.output)
			expected := append(setKeypathOutputs, prefixedOutputs...)
			expected = filterFixtureOutputsToDirectDescendantsOf(test.iterKeypath, expected...)

			iter := state.ChildIterator(test.iterKeypath, false, 0)
			defer iter.Close()
			var i int
			for iter.Rewind(); iter.Valid(); iter.Next() {
				node := iter.Node()
				require.Equal(T, expected[i].keypath, node.Keypath())
				i++
			}
			require.Equal(T, len(expected), i)

		})
	}
}

func TestMemoryNode_DepthFirstIterator(T *testing.T) {
	tests := []struct {
		name        string
		setKeypath  Keypath
		iterKeypath Keypath
		fixture     fixture
	}{
		{"root set, root iter, map value", Keypath(nil), Keypath(nil), fixture1},
		{"root set, root iter, map value 2", Keypath(nil), Keypath(nil), fixture2},
		{"root set, root iter, float value", Keypath(nil), Keypath(nil), fixture5},
		{"root set, root iter, string value", Keypath(nil), Keypath(nil), fixture6},
		{"root set, root iter, bool value", Keypath(nil), Keypath(nil), fixture7},

		{"non-root set, root iter, map value", Keypath("foo/bar"), Keypath(nil), fixture1},
		{"non-root set, root iter, map value 2", Keypath("foo/bar"), Keypath(nil), fixture2},
		{"non-root set, root iter, float value", Keypath("foo/bar"), Keypath(nil), fixture5},
		{"non-root set, root iter, string value", Keypath("foo/bar"), Keypath(nil), fixture6},
		{"non-root set, root iter, bool value", Keypath("foo/bar"), Keypath(nil), fixture7},

		{"root set, non-root iter, map value", Keypath(nil), Keypath("flox"), fixture1},
		{"root set, non-root iter, map value 2", Keypath(nil), Keypath("eee"), fixture2},
		{"root set, non-root iter, float value", Keypath(nil), Keypath("flox"), fixture5},
		{"root set, non-root iter, string value", Keypath(nil), Keypath("flox"), fixture6},
		{"root set, non-root iter, bool value", Keypath(nil), Keypath("flox"), fixture7},

		{"non-root set, non-root iter, map value", Keypath("foo/bar"), Keypath("foo/bar/flox"), fixture1},
		{"non-root set, non-root iter, map value 2", Keypath("foo/bar"), Keypath("foo/bar/eee"), fixture2},
		{"non-root set, non-root iter, float value", Keypath("foo/bar"), Keypath("foo/bar"), fixture5},
		{"non-root set, non-root iter, string value", Keypath("foo/bar"), Keypath("foo/bar"), fixture6},
		{"non-root set, non-root iter, bool value", Keypath("foo/bar"), Keypath("foo/bar"), fixture7},
		{"non-root set, non-root iter, nonexistent value", Keypath("foo/bar"), Keypath("foo/bar/asdf"), fixture7},
	}

	for _, test := range tests {
		test := test
		T.Run(test.name, func(T *testing.T) {
			state := NewMemoryNode()
			state.Set(test.setKeypath, nil, test.fixture.input)

			prefixOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			valueOutputs := combineFixtureOutputs(test.setKeypath, test.fixture)
			expected := append(prefixOutputs, valueOutputs...)
			expected = filterFixtureOutputsWithPrefix(test.iterKeypath, expected...)
			expected = reverseFixtureOutputs(expected...)

			iter := state.DepthFirstIterator(test.iterKeypath, false, 0)
			defer iter.Close()
			var i int
			for iter.Rewind(); iter.Valid(); iter.Next() {
				node := iter.Node()
				require.Equal(T, expected[i].keypath, node.Keypath())
				i++
			}
			require.Equal(T, len(expected), i)

		})
	}
}

func BenchmarkDepthFirstIterator(b *testing.B) {
	t := NewMemoryNode()
	t.Set(Keypath("foo/bar/xyzzy"), nil, []interface{}{1, 2, 3})
	t.Set(Keypath("foo/hello"), nil, "hey there")
	t.Set(Keypath("bling/blang"), nil, map[string]interface{}{"name": "bryn", "age": 33})
	iter := t.DepthFirstIterator(nil, false, 0)
	for n := 0; n < b.N; n++ {
		a := []Keypath{}
		for {
			iter.Next()
			node := iter.Node()
			if node == nil {
				break
			}
			a = append(a, node.(*MemoryNode).keypath)
		}
	}
}

func BenchmarkDepthFirstNativeGoWalk(b *testing.B) {
	x := M{
		"foo": M{
			"bar": M{
				"xyzzy": []interface{}{1, 2, 3},
			},
			"hello": "hey there",
		},
		"bling": M{
			"blang": M{
				"name": "bryn",
				"age":  33,
			},
		},
	}

	for n := 0; n < b.N; n++ {
		a := [][]string{}
		walkTree(x, func(keypath []string, val interface{}) error {
			a = append(a, keypath)
			return nil
		})
	}
}

func walkTree(tree interface{}, fn func(keypath []string, val interface{}) error) error {
	type item struct {
		val     interface{}
		keypath []string
	}

	stack := []item{{val: tree, keypath: []string{}}}
	var current item

	for len(stack) > 0 {
		current = stack[0]
		stack = stack[1:]

		err := fn(current.keypath, current.val)
		if err != nil {
			return err
		}

		if asMap, isMap := current.val.(map[string]interface{}); isMap {
			for key := range asMap {
				kp := make([]string, len(current.keypath)+1)
				copy(kp, current.keypath)
				kp[len(kp)-1] = key
				stack = append(stack, item{
					val:     asMap[key],
					keypath: kp,
				})
			}

		} else if asSlice, isSlice := current.val.([]interface{}); isSlice {
			for i := range asSlice {
				kp := make([]string, len(current.keypath)+1)
				copy(kp, current.keypath)
				kp[len(kp)-1] = strconv.Itoa(i)
				stack = append(stack, item{
					val:     asSlice[i],
					keypath: kp,
				})
			}
		}
	}
	return nil
}

func TestMemoryNode_CopyToMemory(T *testing.T) {
	//state := NewMemoryNode(NodeTypeMap).(*MemoryNode)
	//
	//_, err := state.Set(nil, nil, testVal1)
	//require.NoError(T, err)
	//
	//expectedKeypaths := []Keypath{
	//    Keypath("flox"),
	//    Keypath("flox").PushIndex(0),
	//    Keypath("flox").PushIndex(1),
	//    Keypath("flox").PushIndex(1).Push(Keypath("yup")),
	//    Keypath("flox").PushIndex(1).Push(Keypath("hey")),
	//    Keypath("flox").PushIndex(2),
	//}
	//
	//sort.Slice(expectedKeypaths, func(i, j int) bool { return bytes.Compare(expectedKeypaths[i], expectedKeypaths[j]) < 0 })
	//
	//err = state.View(func(node *DBNode) error {
	//    copied, err := node.NodeAt(Keypath("flox")).CopyToMemory(nil)
	//    require.NoError(T, err)
	//
	//    memnode := copied.(*MemoryNode)
	//    for i := range memnode.keypaths {
	//        require.Equal(T, expectedKeypaths[i], memnode.keypaths[i])
	//    }
	//    return nil
	//})
	//require.NoError(T, err)
}

func TestMemoryNode_CopyToMemory_AtKeypath(T *testing.T) {
	node := NewMemoryNode()
	err := node.Set(nil, nil, fixture1.input)
	require.NoError(T, err)

	copied, err := node.NodeAt(Keypath("asdf"), nil).CopyToMemory(nil, nil)
	require.NoError(T, err)

	expected := filterFixtureOutputsWithPrefix(Keypath("asdf"), fixture1.output...)

	memnode := copied.(*MemoryNode)
	for i, kp := range memnode.keypaths {
		require.Equal(T, expected[i].keypath, kp)
		require.Equal(T, expected[i].nodeType, memnode.nodeTypes[string(kp)])
		if memnode.nodeTypes[string(kp)] == NodeTypeSlice || memnode.nodeTypes[string(kp)] == NodeTypeMap {
			require.Equal(T, len(expected[i].value.([]interface{})), memnode.contentLengths[string(kp)])
		}
	}
	require.NoError(T, err)
}
