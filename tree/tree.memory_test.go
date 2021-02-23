package tree_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"redwood.dev/tree"
)

func TestMemoryNode_Set(t *testing.T) {
	tests := []struct {
		name       string
		setKeypath tree.Keypath
		atKeypath  tree.Keypath
		rng        *tree.Range
		fixture    fixture
	}{
		{"root keypath, single set, map value", tree.Keypath(nil), tree.Keypath(nil), nil, fixture1},
		{"root keypath, single set, map value 2", tree.Keypath(nil), tree.Keypath(nil), nil, fixture2},
		{"root keypath, single set, float value", tree.Keypath(nil), tree.Keypath(nil), nil, fixture5},
		{"root keypath, single set, string value", tree.Keypath(nil), tree.Keypath(nil), nil, fixture6},
		{"root keypath, single set, bool value", tree.Keypath(nil), tree.Keypath(nil), nil, fixture7},

		{"non-root keypath, single set, map value", tree.Keypath("foo/bar"), tree.Keypath(nil), nil, fixture1},
		{"non-root keypath, single set, map value 2", tree.Keypath("foo/bar"), tree.Keypath(nil), nil, fixture2},
		{"non-root keypath, single set, float value", tree.Keypath("foo/bar"), tree.Keypath(nil), nil, fixture5},
		{"non-root keypath, single set, string value", tree.Keypath("foo/bar"), tree.Keypath(nil), nil, fixture6},
		{"non-root keypath, single set, bool value", tree.Keypath("foo/bar"), tree.Keypath(nil), nil, fixture7},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			state := tree.NewMemoryNode().NodeAt(test.atKeypath, nil).(*tree.MemoryNode)

			err := state.Set(test.setKeypath, test.rng, test.fixture.input)
			require.NoError(t, err)

			prefixOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			expectedNodesFromSetKeypath := len(prefixOutputs)

			expectedNumValues := countNodesOfType(tree.NodeTypeValue, test.fixture.output)
			expectedNumSlices := countNodesOfType(tree.NodeTypeSlice, test.fixture.output)
			expectedNumMaps := countNodesOfType(tree.NodeTypeMap, test.fixture.output) + expectedNodesFromSetKeypath
			expectedNumNodes := expectedNumValues + expectedNumSlices + expectedNumMaps
			require.Equal(t, expectedNumValues, len(state.Values()))
			require.Equal(t, expectedNumNodes, len(state.NodeTypes()))
			require.Equal(t, expectedNumNodes, len(state.Keypaths()))

			expected := prefixFixtureOutputs(test.setKeypath, test.fixture.output)
			expected = append(prefixOutputs, expected...)

			for i, kp := range state.Keypaths() {
				require.Equal(t, expected[i].keypath, kp)
				require.Equal(t, expected[i].nodeType, state.NodeTypes()[string(kp)])
				if state.NodeTypes()[string(kp)] == tree.NodeTypeValue {
					require.Equal(t, expected[i].value, state.Values()[string(kp)])
				}
			}
		})
	}
}

func TestMemoryNode_SetNode(t *testing.T) {
	t.Run("inner tree.MemoryNode", func(t *testing.T) {
		state := tree.NewMemoryNode().(*tree.MemoryNode)
		innerNode := tree.NewMemoryNode().(*tree.MemoryNode)

		err := state.Set(tree.Keypath("foo"), nil, M{
			"one":   uint64(123),
			"two":   M{"two a": false, "two b": nil},
			"three": "hello",
		})
		require.NoError(t, err)

		err = innerNode.Set(tree.Keypath("bar"), nil, M{
			"baz": M{
				"xyzzy": "zork",
			},
		})
		require.NoError(t, err)

		err = state.Set(tree.Keypath("foo/two"), nil, innerNode)
		require.NoError(t, err)

		require.Len(t, state.Keypaths(), 5)

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
		require.NoError(t, err)
		require.True(t, exists)
		require.Equal(t, expected, received)
	})
}

func TestMemoryNode_ParentNodeFor(t *testing.T) {
	t.Run("non-root node", func(t *testing.T) {
		state := tree.NewMemoryNode().(*tree.MemoryNode)
		innerNode := tree.NewMemoryNode().(*tree.MemoryNode)

		err := state.Set(tree.Keypath("foo"), nil, M{
			"one":   uint64(123),
			"two":   M{"two a": false, "two b": nil},
			"three": "hello",
		})
		require.NoError(t, err)

		err = innerNode.Set(tree.Keypath("bar"), nil, M{
			"baz": M{
				"xyzzy": "zork",
			},
		})
		require.NoError(t, err)

		err = state.Set(tree.Keypath("foo/two"), nil, innerNode)
		require.NoError(t, err)

		node, keypath := state.ParentNodeFor(tree.Keypath("foo/two/bar/baz/xyzzy"))
		require.Equal(t, tree.Keypath("bar/baz/xyzzy"), keypath)
		memNode := node.(*tree.MemoryNode)
		require.Equal(t, innerNode.Keypaths(), memNode.Keypaths())
	})

	t.Run("root node", func(t *testing.T) {
		state := tree.NewMemoryNode().(*tree.MemoryNode)
		innerNode := tree.NewMemoryNode().(*tree.MemoryNode)

		err := state.Set(tree.Keypath("foo"), nil, M{
			"one":   uint64(123),
			"two":   M{"two a": false, "two b": nil},
			"three": "hello",
		})
		require.NoError(t, err)

		err = innerNode.Set(tree.Keypath("bar"), nil, M{
			"baz": M{
				"xyzzy": "zork",
			},
		})
		require.NoError(t, err)

		err = state.Set(nil, nil, innerNode)
		require.NoError(t, err)

		node, keypath := state.ParentNodeFor(tree.Keypath("bar/baz/xyzzy"))
		require.Equal(t, tree.Keypath("bar/baz/xyzzy"), keypath)
		memNode := node.(*tree.MemoryNode)
		require.Equal(t, innerNode.Keypaths(), memNode.Keypaths())
	})
}

func TestMemoryNode_Delete(t *testing.T) {
	tests := []struct {
		name          string
		setKeypath    tree.Keypath
		setRange      *tree.Range
		deleteKeypath tree.Keypath
		deleteRange   *tree.Range
		fixture       fixture
	}{
		{"root keypath, single set, map value", tree.Keypath(nil), nil, tree.Keypath("flox"), nil, fixture1},
		{"root keypath, single set, map value 2", tree.Keypath(nil), nil, tree.Keypath("eee"), nil, fixture2},
		{"root keypath, single set, float value", tree.Keypath(nil), nil, tree.Keypath(nil), nil, fixture5},
		{"root keypath, single set, string value", tree.Keypath(nil), nil, tree.Keypath(nil), nil, fixture6},
		{"root keypath, single set, bool value", tree.Keypath(nil), nil, tree.Keypath(nil), nil, fixture7},

		{"non-root keypath, single set, map value", tree.Keypath("foo/bar"), nil, tree.Keypath(nil), nil, fixture1},
		{"non-root keypath, single set, map value 2", tree.Keypath("foo/bar"), nil, tree.Keypath(nil), nil, fixture2},
		{"non-root keypath, single set, float value", tree.Keypath("foo/bar"), nil, tree.Keypath(nil), nil, fixture5},
		{"non-root keypath, single set, string value", tree.Keypath("foo/bar"), nil, tree.Keypath(nil), nil, fixture6},
		{"non-root keypath, single set, bool value", tree.Keypath("foo/bar"), nil, tree.Keypath(nil), nil, fixture7},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			state := tree.NewMemoryNode().(*tree.MemoryNode)

			err := state.Set(test.setKeypath, test.setRange, test.fixture.input)
			require.NoError(t, err)

			err = state.Delete(test.deleteKeypath, test.deleteRange)
			require.NoError(t, err)

			remainingOutputs := removeFixtureOutputsWithPrefix(test.deleteKeypath, test.fixture.output...)

			// expectedNumValues := countNodesOfType(tree.NodeTypeValue, remainingOutputs)
			// expectedNumSlices := countNodesOfType(tree.NodeTypeSlice, remainingOutputs)
			// expectedNumMaps := countNodesOfType(tree.NodeTypeMap, remainingOutputs) + test.deleteKeypath.NumParts()
			// expectedNumNodes := expectedNumValues + expectedNumSlices + expectedNumMaps
			// require.Equal(t, expectedNumValues, len(state.Values()))
			// require.Equal(t, expectedNumNodes, len(state.NodeTypes()))
			// require.Equal(t, expectedNumNodes, len(state.Keypaths()))

			setKeypathOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			expected := append(setKeypathOutputs, remainingOutputs...)
			for i, kp := range state.Keypaths() {
				require.Equal(t, expected[i].keypath, kp)
				require.Equal(t, expected[i].nodeType, state.NodeTypes()[string(kp)])
				if state.NodeTypes()[string(kp)] == tree.NodeTypeValue {
					require.Equal(t, expected[i].value, state.Values()[string(kp)])
				}
			}
		})
	}
}

func TestMemoryNode_Iterator(t *testing.T) {
	tests := []struct {
		name        string
		setKeypath  tree.Keypath
		iterKeypath tree.Keypath
		fixture     fixture
	}{
		{"root set, root iter, map value", tree.Keypath(nil), tree.Keypath(nil), fixture1},
		{"root set, root iter, map value 2", tree.Keypath(nil), tree.Keypath(nil), fixture2},
		{"root set, root iter, float value", tree.Keypath(nil), tree.Keypath(nil), fixture5},
		{"root set, root iter, string value", tree.Keypath(nil), tree.Keypath(nil), fixture6},
		{"root set, root iter, bool value", tree.Keypath(nil), tree.Keypath(nil), fixture7},

		{"non-root set, root iter, map value", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture1},
		{"non-root set, root iter, map value 2", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture2},
		{"non-root set, root iter, float value", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture5},
		{"non-root set, root iter, string value", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture6},
		{"non-root set, root iter, bool value", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture7},

		{"root set, non-root iter, map value", tree.Keypath(nil), tree.Keypath("flox"), fixture1},
		{"root set, non-root iter, map value 2", tree.Keypath(nil), tree.Keypath("flox"), fixture2},
		{"root set, non-root iter, float value", tree.Keypath(nil), tree.Keypath("flox"), fixture5},
		{"root set, non-root iter, string value", tree.Keypath(nil), tree.Keypath("flox"), fixture6},
		{"root set, non-root iter, bool value", tree.Keypath(nil), tree.Keypath("flox"), fixture7},

		{"non-root set, non-root iter, map value", tree.Keypath("foo/bar"), tree.Keypath("flox"), fixture1},
		{"non-root set, non-root iter, map value 2", tree.Keypath("foo/bar"), tree.Keypath("flox"), fixture2},
		{"non-root set, non-root iter, float value", tree.Keypath("foo/bar"), tree.Keypath("flox"), fixture5},
		{"non-root set, non-root iter, string value", tree.Keypath("foo/bar"), tree.Keypath("flox"), fixture6},
		{"non-root set, non-root iter, bool value", tree.Keypath("foo/bar"), tree.Keypath("flox"), fixture7},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			state := tree.NewMemoryNode()
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
				require.Equal(t, expected[i].keypath, node.Keypath())
				i++
			}
			require.Equal(t, len(expected), i)

		})
	}
}

func TestMemoryNode_ChildIterator(t *testing.T) {
	tests := []struct {
		name        string
		setKeypath  tree.Keypath
		iterKeypath tree.Keypath
		fixture     fixture
	}{
		{"root set, root iter, map value", tree.Keypath(nil), tree.Keypath(nil), fixture1},
		{"root set, root iter, map value 2", tree.Keypath(nil), tree.Keypath(nil), fixture2},
		{"root set, root iter, float value", tree.Keypath(nil), tree.Keypath(nil), fixture5},
		{"root set, root iter, string value", tree.Keypath(nil), tree.Keypath(nil), fixture6},
		{"root set, root iter, bool value", tree.Keypath(nil), tree.Keypath(nil), fixture7},

		{"non-root set, root iter, map value", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture1},
		{"non-root set, root iter, map value 2", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture2},
		{"non-root set, root iter, float value", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture5},
		{"non-root set, root iter, string value", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture6},
		{"non-root set, root iter, bool value", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture7},

		{"root set, non-root iter, map value", tree.Keypath(nil), tree.Keypath("flox"), fixture1},
		{"root set, non-root iter, map value 2", tree.Keypath(nil), tree.Keypath("flox"), fixture2},
		{"root set, non-root iter, float value", tree.Keypath(nil), tree.Keypath("flox"), fixture5},
		{"root set, non-root iter, string value", tree.Keypath(nil), tree.Keypath("flox"), fixture6},
		{"root set, non-root iter, bool value", tree.Keypath(nil), tree.Keypath("flox"), fixture7},

		{"non-root set, non-root iter, map value", tree.Keypath("foo/bar"), tree.Keypath("flox"), fixture1},
		{"non-root set, non-root iter, map value 2", tree.Keypath("foo/bar"), tree.Keypath("flox"), fixture2},
		{"non-root set, non-root iter, float value", tree.Keypath("foo/bar"), tree.Keypath("flox"), fixture5},
		{"non-root set, non-root iter, string value", tree.Keypath("foo/bar"), tree.Keypath("flox"), fixture6},
		{"non-root set, non-root iter, bool value", tree.Keypath("foo/bar"), tree.Keypath("flox"), fixture7},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			state := tree.NewMemoryNode()
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
				require.Equal(t, expected[i].keypath, node.Keypath())
				i++
			}
			require.Equal(t, len(expected), i)

		})
	}
}

func TestMemoryNode_DepthFirstIterator(t *testing.T) {
	tests := []struct {
		name        string
		setKeypath  tree.Keypath
		iterKeypath tree.Keypath
		fixture     fixture
	}{
		{"root set, root iter, map value", tree.Keypath(nil), tree.Keypath(nil), fixture1},
		{"root set, root iter, map value 2", tree.Keypath(nil), tree.Keypath(nil), fixture2},
		{"root set, root iter, float value", tree.Keypath(nil), tree.Keypath(nil), fixture5},
		{"root set, root iter, string value", tree.Keypath(nil), tree.Keypath(nil), fixture6},
		{"root set, root iter, bool value", tree.Keypath(nil), tree.Keypath(nil), fixture7},

		{"non-root set, root iter, map value", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture1},
		{"non-root set, root iter, map value 2", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture2},
		{"non-root set, root iter, float value", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture5},
		{"non-root set, root iter, string value", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture6},
		{"non-root set, root iter, bool value", tree.Keypath("foo/bar"), tree.Keypath(nil), fixture7},

		{"root set, non-root iter, map value", tree.Keypath(nil), tree.Keypath("flox"), fixture1},
		{"root set, non-root iter, map value 2", tree.Keypath(nil), tree.Keypath("eee"), fixture2},
		{"root set, non-root iter, float value", tree.Keypath(nil), tree.Keypath("flox"), fixture5},
		{"root set, non-root iter, string value", tree.Keypath(nil), tree.Keypath("flox"), fixture6},
		{"root set, non-root iter, bool value", tree.Keypath(nil), tree.Keypath("flox"), fixture7},

		{"non-root set, non-root iter, map value", tree.Keypath("foo/bar"), tree.Keypath("foo/bar/flox"), fixture1},
		{"non-root set, non-root iter, map value 2", tree.Keypath("foo/bar"), tree.Keypath("foo/bar/eee"), fixture2},
		{"non-root set, non-root iter, float value", tree.Keypath("foo/bar"), tree.Keypath("foo/bar"), fixture5},
		{"non-root set, non-root iter, string value", tree.Keypath("foo/bar"), tree.Keypath("foo/bar"), fixture6},
		{"non-root set, non-root iter, bool value", tree.Keypath("foo/bar"), tree.Keypath("foo/bar"), fixture7},
		{"non-root set, non-root iter, nonexistent value", tree.Keypath("foo/bar"), tree.Keypath("foo/bar/asdf"), fixture7},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			state := tree.NewMemoryNode()
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
				require.Equal(t, expected[i].keypath, node.Keypath())
				i++
			}
			require.Equal(t, len(expected), i)

		})
	}
}

func BenchmarkDepthFirstIterator(b *testing.B) {
	t := tree.NewMemoryNode()
	t.Set(tree.Keypath("foo/bar/xyzzy"), nil, []interface{}{1, 2, 3})
	t.Set(tree.Keypath("foo/hello"), nil, "hey there")
	t.Set(tree.Keypath("bling/blang"), nil, map[string]interface{}{"name": "bryn", "age": 33})
	iter := t.DepthFirstIterator(nil, false, 0)
	for n := 0; n < b.N; n++ {
		a := []tree.Keypath{}
		for {
			iter.Next()
			node := iter.Node()
			if node == nil {
				break
			}
			a = append(a, node.(*tree.MemoryNode).Keypath())
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

func TestMemoryNode_CopyToMemory(t *testing.T) {
	//state := tree.NewMemoryNode(tree.NodeTypeMap).(*tree.MemoryNode)
	//
	//_, err := state.Set(nil, nil, testVal1)
	//require.NoError(t, err)
	//
	//expectedKeypaths := []tree.Keypath{
	//    tree.Keypath("flox"),
	//    tree.Keypath("flox").PushIndex(0),
	//    tree.Keypath("flox").PushIndex(1),
	//    tree.Keypath("flox").PushIndex(1).Push(tree.Keypath("yup")),
	//    tree.Keypath("flox").PushIndex(1).Push(tree.Keypath("hey")),
	//    tree.Keypath("flox").PushIndex(2),
	//}
	//
	//sort.Slice(expectedKeypaths, func(i, j int) bool { return bytes.Compare(expectedKeypaths[i], expectedKeypaths[j]) < 0 })
	//
	//err = state.View(func(node *DBNode) error {
	//    copied, err := node.NodeAt(tree.Keypath("flox")).CopyToMemory(nil)
	//    require.NoError(t, err)
	//
	//    memnode := copied.(*tree.MemoryNode)
	//    for i := range memnode.Keypaths() {
	//        require.Equal(t, expectedKeypaths[i], memnode.Keypaths()[i])
	//    }
	//    return nil
	//})
	//require.NoError(t, err)
}

func TestMemoryNode_CopyToMemory_AtKeypath(t *testing.T) {
	node := tree.NewMemoryNode()
	err := node.Set(nil, nil, fixture1.input)
	require.NoError(t, err)

	copied, err := node.NodeAt(tree.Keypath("asdf"), nil).CopyToMemory(nil, nil)
	require.NoError(t, err)

	expected := filterFixtureOutputsWithPrefix(tree.Keypath("asdf"), fixture1.output...)

	memnode := copied.(*tree.MemoryNode)
	for i, kp := range memnode.Keypaths() {
		require.Equal(t, expected[i].keypath, kp)
		require.Equal(t, expected[i].nodeType, memnode.NodeTypes()[string(kp)])
		if memnode.NodeTypes()[string(kp)] == tree.NodeTypeSlice || memnode.NodeTypes()[string(kp)] == tree.NodeTypeMap {
			require.Equal(t, uint64(len(expected[i].value.([]interface{}))), memnode.ContentLengths()[string(kp)])
		}
	}
	require.NoError(t, err)
}
