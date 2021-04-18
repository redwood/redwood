package state_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/state"
)

func TestMemoryNode_Set(t *testing.T) {
	tests := []struct {
		name       string
		setKeypath state.Keypath
		atKeypath  state.Keypath
		rng        *state.Range
		fixture    fixture
	}{
		{"root keypath, single set, map value", state.Keypath(nil), state.Keypath(nil), nil, fixture1},
		{"root keypath, single set, map value 2", state.Keypath(nil), state.Keypath(nil), nil, fixture2},
		{"root keypath, single set, float value", state.Keypath(nil), state.Keypath(nil), nil, fixture5},
		{"root keypath, single set, string value", state.Keypath(nil), state.Keypath(nil), nil, fixture6},
		{"root keypath, single set, bool value", state.Keypath(nil), state.Keypath(nil), nil, fixture7},

		{"non-root keypath, single set, map value", state.Keypath("foo/bar"), state.Keypath(nil), nil, fixture1},
		{"non-root keypath, single set, map value 2", state.Keypath("foo/bar"), state.Keypath(nil), nil, fixture2},
		{"non-root keypath, single set, float value", state.Keypath("foo/bar"), state.Keypath(nil), nil, fixture5},
		{"non-root keypath, single set, string value", state.Keypath("foo/bar"), state.Keypath(nil), nil, fixture6},
		{"non-root keypath, single set, bool value", state.Keypath("foo/bar"), state.Keypath(nil), nil, fixture7},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			node := state.NewMemoryNode().NodeAt(test.atKeypath, nil).(*state.MemoryNode)

			err := node.Set(test.setKeypath, test.rng, test.fixture.input)
			require.NoError(t, err)

			prefixOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			expectedNodesFromSetKeypath := len(prefixOutputs)

			expectedNumValues := countNodesOfType(state.NodeTypeValue, test.fixture.output)
			expectedNumSlices := countNodesOfType(state.NodeTypeSlice, test.fixture.output)
			expectedNumMaps := countNodesOfType(state.NodeTypeMap, test.fixture.output) + expectedNodesFromSetKeypath
			expectedNumNodes := expectedNumValues + expectedNumSlices + expectedNumMaps
			require.Equal(t, expectedNumValues, len(node.Values()))
			require.Equal(t, expectedNumNodes, len(node.NodeTypes()))
			require.Equal(t, expectedNumNodes, len(node.Keypaths()))

			expected := prefixFixtureOutputs(test.setKeypath, test.fixture.output)
			expected = append(prefixOutputs, expected...)

			for i, kp := range node.Keypaths() {
				require.Equal(t, expected[i].keypath, kp)
				require.Equal(t, expected[i].nodeType, node.NodeTypes()[string(kp)])
				if node.NodeTypes()[string(kp)] == state.NodeTypeValue {
					require.Equal(t, expected[i].value, node.Values()[string(kp)])
				}
			}
		})
	}
}

func TestMemoryNode_SetNode(t *testing.T) {
	t.Run("inner state.MemoryNode", func(t *testing.T) {
		node := state.NewMemoryNode().(*state.MemoryNode)
		innerNode := state.NewMemoryNode().(*state.MemoryNode)

		err := node.Set(state.Keypath("foo"), nil, M{
			"one":   uint64(123),
			"two":   M{"two a": false, "two b": nil},
			"three": "hello",
		})
		require.NoError(t, err)

		err = innerNode.Set(state.Keypath("bar"), nil, M{
			"baz": M{
				"xyzzy": "zork",
			},
		})
		require.NoError(t, err)

		err = node.Set(state.Keypath("foo/two"), nil, innerNode)
		require.NoError(t, err)

		require.Len(t, node.Keypaths(), 5)

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

		received, exists, err := node.Value(nil, nil)
		require.NoError(t, err)
		require.True(t, exists)
		require.Equal(t, expected, received)
	})
}

func TestMemoryNode_ParentNodeFor(t *testing.T) {
	t.Run("non-root node", func(t *testing.T) {
		node := state.NewMemoryNode().(*state.MemoryNode)
		innerNode := state.NewMemoryNode().(*state.MemoryNode)

		err := node.Set(state.Keypath("foo"), nil, M{
			"one":   uint64(123),
			"two":   M{"two a": false, "two b": nil},
			"three": "hello",
		})
		require.NoError(t, err)

		err = innerNode.Set(state.Keypath("bar"), nil, M{
			"baz": M{
				"xyzzy": "zork",
			},
		})
		require.NoError(t, err)

		err = node.Set(state.Keypath("foo/two"), nil, innerNode)
		require.NoError(t, err)

		parent, keypath := node.ParentNodeFor(state.Keypath("foo/two/bar/baz/xyzzy"))
		require.Equal(t, state.Keypath("bar/baz/xyzzy"), keypath)
		memNode := parent.(*state.MemoryNode)
		require.Equal(t, innerNode.Keypaths(), memNode.Keypaths())
	})

	t.Run("root node", func(t *testing.T) {
		node := state.NewMemoryNode().(*state.MemoryNode)
		innerNode := state.NewMemoryNode().(*state.MemoryNode)

		err := node.Set(state.Keypath("foo"), nil, M{
			"one":   uint64(123),
			"two":   M{"two a": false, "two b": nil},
			"three": "hello",
		})
		require.NoError(t, err)

		err = innerNode.Set(state.Keypath("bar"), nil, M{
			"baz": M{
				"xyzzy": "zork",
			},
		})
		require.NoError(t, err)

		err = node.Set(nil, nil, innerNode)
		require.NoError(t, err)

		parent, keypath := node.ParentNodeFor(state.Keypath("bar/baz/xyzzy"))
		require.Equal(t, state.Keypath("bar/baz/xyzzy"), keypath)
		memNode := parent.(*state.MemoryNode)
		require.Equal(t, innerNode.Keypaths(), memNode.Keypaths())
	})
}

func TestMemoryNode_Delete(t *testing.T) {
	tests := []struct {
		name          string
		setKeypath    state.Keypath
		setRange      *state.Range
		deleteKeypath state.Keypath
		deleteRange   *state.Range
		fixture       fixture
	}{
		{"root keypath, single set, map value", state.Keypath(nil), nil, state.Keypath("flox"), nil, fixture1},
		{"root keypath, single set, map value 2", state.Keypath(nil), nil, state.Keypath("eee"), nil, fixture2},
		{"root keypath, single set, float value", state.Keypath(nil), nil, state.Keypath(nil), nil, fixture5},
		{"root keypath, single set, string value", state.Keypath(nil), nil, state.Keypath(nil), nil, fixture6},
		{"root keypath, single set, bool value", state.Keypath(nil), nil, state.Keypath(nil), nil, fixture7},

		{"non-root keypath, single set, map value", state.Keypath("foo/bar"), nil, state.Keypath(nil), nil, fixture1},
		{"non-root keypath, single set, map value 2", state.Keypath("foo/bar"), nil, state.Keypath(nil), nil, fixture2},
		{"non-root keypath, single set, float value", state.Keypath("foo/bar"), nil, state.Keypath(nil), nil, fixture5},
		{"non-root keypath, single set, string value", state.Keypath("foo/bar"), nil, state.Keypath(nil), nil, fixture6},
		{"non-root keypath, single set, bool value", state.Keypath("foo/bar"), nil, state.Keypath(nil), nil, fixture7},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			node := state.NewMemoryNode().(*state.MemoryNode)

			err := node.Set(test.setKeypath, test.setRange, test.fixture.input)
			require.NoError(t, err)

			err = node.Delete(test.deleteKeypath, test.deleteRange)
			require.NoError(t, err)

			remainingOutputs := removeFixtureOutputsWithPrefix(test.deleteKeypath, test.fixture.output...)

			// expectedNumValues := countNodesOfType(state.NodeTypeValue, remainingOutputs)
			// expectedNumSlices := countNodesOfType(state.NodeTypeSlice, remainingOutputs)
			// expectedNumMaps := countNodesOfType(state.NodeTypeMap, remainingOutputs) + test.deleteKeypath.NumParts()
			// expectedNumNodes := expectedNumValues + expectedNumSlices + expectedNumMaps
			// require.Equal(t, expectedNumValues, len(node.Values()))
			// require.Equal(t, expectedNumNodes, len(node.NodeTypes()))
			// require.Equal(t, expectedNumNodes, len(node.Keypaths()))

			setKeypathOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			expected := append(setKeypathOutputs, remainingOutputs...)
			for i, kp := range node.Keypaths() {
				require.Equal(t, expected[i].keypath, kp)
				require.Equal(t, expected[i].nodeType, node.NodeTypes()[string(kp)])
				if node.NodeTypes()[string(kp)] == state.NodeTypeValue {
					require.Equal(t, expected[i].value, node.Values()[string(kp)])
				}
			}
		})
	}
}

func TestMemoryNode_Iterator(t *testing.T) {
	tests := []struct {
		name        string
		setKeypath  state.Keypath
		iterKeypath state.Keypath
		fixture     fixture
	}{
		{"root set, root iter, map value", state.Keypath(nil), state.Keypath(nil), fixture1},
		{"root set, root iter, map value 2", state.Keypath(nil), state.Keypath(nil), fixture2},
		{"root set, root iter, float value", state.Keypath(nil), state.Keypath(nil), fixture5},
		{"root set, root iter, string value", state.Keypath(nil), state.Keypath(nil), fixture6},
		{"root set, root iter, bool value", state.Keypath(nil), state.Keypath(nil), fixture7},

		{"non-root set, root iter, map value", state.Keypath("foo/bar"), state.Keypath(nil), fixture1},
		{"non-root set, root iter, map value 2", state.Keypath("foo/bar"), state.Keypath(nil), fixture2},
		{"non-root set, root iter, float value", state.Keypath("foo/bar"), state.Keypath(nil), fixture5},
		{"non-root set, root iter, string value", state.Keypath("foo/bar"), state.Keypath(nil), fixture6},
		{"non-root set, root iter, bool value", state.Keypath("foo/bar"), state.Keypath(nil), fixture7},

		{"root set, non-root iter, map value", state.Keypath(nil), state.Keypath("flox"), fixture1},
		{"root set, non-root iter, map value 2", state.Keypath(nil), state.Keypath("flox"), fixture2},
		{"root set, non-root iter, float value", state.Keypath(nil), state.Keypath("flox"), fixture5},
		{"root set, non-root iter, string value", state.Keypath(nil), state.Keypath("flox"), fixture6},
		{"root set, non-root iter, bool value", state.Keypath(nil), state.Keypath("flox"), fixture7},

		{"non-root set, non-root iter, map value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture1},
		{"non-root set, non-root iter, map value 2", state.Keypath("foo/bar"), state.Keypath("flox"), fixture2},
		{"non-root set, non-root iter, float value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture5},
		{"non-root set, non-root iter, string value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture6},
		{"non-root set, non-root iter, bool value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture7},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			node := state.NewMemoryNode()
			node.Set(test.setKeypath, nil, test.fixture.input)

			setKeypathOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			valueOutputs := prefixFixtureOutputs(test.setKeypath, test.fixture.output)
			expected := append(setKeypathOutputs, valueOutputs...)
			expected = filterFixtureOutputsWithPrefix(test.iterKeypath, expected...)

			iter := node.Iterator(test.iterKeypath, false, 0)
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
		setKeypath  state.Keypath
		iterKeypath state.Keypath
		fixture     fixture
	}{
		{"root set, root iter, map value", state.Keypath(nil), state.Keypath(nil), fixture1},
		{"root set, root iter, map value 2", state.Keypath(nil), state.Keypath(nil), fixture2},
		{"root set, root iter, float value", state.Keypath(nil), state.Keypath(nil), fixture5},
		{"root set, root iter, string value", state.Keypath(nil), state.Keypath(nil), fixture6},
		{"root set, root iter, bool value", state.Keypath(nil), state.Keypath(nil), fixture7},

		{"non-root set, root iter, map value", state.Keypath("foo/bar"), state.Keypath(nil), fixture1},
		{"non-root set, root iter, map value 2", state.Keypath("foo/bar"), state.Keypath(nil), fixture2},
		{"non-root set, root iter, float value", state.Keypath("foo/bar"), state.Keypath(nil), fixture5},
		{"non-root set, root iter, string value", state.Keypath("foo/bar"), state.Keypath(nil), fixture6},
		{"non-root set, root iter, bool value", state.Keypath("foo/bar"), state.Keypath(nil), fixture7},

		{"root set, non-root iter, map value", state.Keypath(nil), state.Keypath("flox"), fixture1},
		{"root set, non-root iter, map value 2", state.Keypath(nil), state.Keypath("flox"), fixture2},
		{"root set, non-root iter, float value", state.Keypath(nil), state.Keypath("flox"), fixture5},
		{"root set, non-root iter, string value", state.Keypath(nil), state.Keypath("flox"), fixture6},
		{"root set, non-root iter, bool value", state.Keypath(nil), state.Keypath("flox"), fixture7},

		{"non-root set, non-root iter, map value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture1},
		{"non-root set, non-root iter, map value 2", state.Keypath("foo/bar"), state.Keypath("flox"), fixture2},
		{"non-root set, non-root iter, float value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture5},
		{"non-root set, non-root iter, string value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture6},
		{"non-root set, non-root iter, bool value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture7},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			node := state.NewMemoryNode()
			node.Set(test.setKeypath, nil, test.fixture.input)

			setKeypathOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			prefixedOutputs := prefixFixtureOutputs(test.setKeypath, test.fixture.output)
			expected := append(setKeypathOutputs, prefixedOutputs...)
			expected = filterFixtureOutputsToDirectDescendantsOf(test.iterKeypath, expected...)

			iter := node.ChildIterator(test.iterKeypath, false, 0)
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
		setKeypath  state.Keypath
		iterKeypath state.Keypath
		fixture     fixture
	}{
		{"root set, root iter, map value", state.Keypath(nil), state.Keypath(nil), fixture1},
		{"root set, root iter, map value 2", state.Keypath(nil), state.Keypath(nil), fixture2},
		{"root set, root iter, float value", state.Keypath(nil), state.Keypath(nil), fixture5},
		{"root set, root iter, string value", state.Keypath(nil), state.Keypath(nil), fixture6},
		{"root set, root iter, bool value", state.Keypath(nil), state.Keypath(nil), fixture7},

		{"non-root set, root iter, map value", state.Keypath("foo/bar"), state.Keypath(nil), fixture1},
		{"non-root set, root iter, map value 2", state.Keypath("foo/bar"), state.Keypath(nil), fixture2},
		{"non-root set, root iter, float value", state.Keypath("foo/bar"), state.Keypath(nil), fixture5},
		{"non-root set, root iter, string value", state.Keypath("foo/bar"), state.Keypath(nil), fixture6},
		{"non-root set, root iter, bool value", state.Keypath("foo/bar"), state.Keypath(nil), fixture7},

		{"root set, non-root iter, map value", state.Keypath(nil), state.Keypath("flox"), fixture1},
		{"root set, non-root iter, map value 2", state.Keypath(nil), state.Keypath("eee"), fixture2},
		{"root set, non-root iter, float value", state.Keypath(nil), state.Keypath("flox"), fixture5},
		{"root set, non-root iter, string value", state.Keypath(nil), state.Keypath("flox"), fixture6},
		{"root set, non-root iter, bool value", state.Keypath(nil), state.Keypath("flox"), fixture7},

		{"non-root set, non-root iter, map value", state.Keypath("foo/bar"), state.Keypath("foo/bar/flox"), fixture1},
		{"non-root set, non-root iter, map value 2", state.Keypath("foo/bar"), state.Keypath("foo/bar/eee"), fixture2},
		{"non-root set, non-root iter, float value", state.Keypath("foo/bar"), state.Keypath("foo/bar"), fixture5},
		{"non-root set, non-root iter, string value", state.Keypath("foo/bar"), state.Keypath("foo/bar"), fixture6},
		{"non-root set, non-root iter, bool value", state.Keypath("foo/bar"), state.Keypath("foo/bar"), fixture7},
		{"non-root set, non-root iter, nonexistent value", state.Keypath("foo/bar"), state.Keypath("foo/bar/asdf"), fixture7},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			node := state.NewMemoryNode()
			node.Set(test.setKeypath, nil, test.fixture.input)

			prefixOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			valueOutputs := combineFixtureOutputs(test.setKeypath, test.fixture)
			expected := append(prefixOutputs, valueOutputs...)
			expected = filterFixtureOutputsWithPrefix(test.iterKeypath, expected...)
			expected = reverseFixtureOutputs(expected...)

			iter := node.DepthFirstIterator(test.iterKeypath, false, 0)
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
	t := state.NewMemoryNode()
	t.Set(state.Keypath("foo/bar/xyzzy"), nil, []interface{}{1, 2, 3})
	t.Set(state.Keypath("foo/hello"), nil, "hey there")
	t.Set(state.Keypath("bling/blang"), nil, map[string]interface{}{"name": "bryn", "age": 33})
	iter := t.DepthFirstIterator(nil, false, 0)
	for n := 0; n < b.N; n++ {
		a := []state.Keypath{}
		for {
			iter.Next()
			node := iter.Node()
			if node == nil {
				break
			}
			a = append(a, node.(*state.MemoryNode).Keypath())
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
	//node := state.NewMemoryNode(state.NodeTypeMap).(*state.MemoryNode)
	//
	//_, err := node.Set(nil, nil, testVal1)
	//require.NoError(t, err)
	//
	//expectedKeypaths := []state.Keypath{
	//    state.Keypath("flox"),
	//    state.Keypath("flox").PushIndex(0),
	//    state.Keypath("flox").PushIndex(1),
	//    state.Keypath("flox").PushIndex(1).Push(state.Keypath("yup")),
	//    state.Keypath("flox").PushIndex(1).Push(state.Keypath("hey")),
	//    state.Keypath("flox").PushIndex(2),
	//}
	//
	//sort.Slice(expectedKeypaths, func(i, j int) bool { return bytes.Compare(expectedKeypaths[i], expectedKeypaths[j]) < 0 })
	//
	//err = state.View(func(node *DBNode) error {
	//    copied, err := node.NodeAt(state.Keypath("flox")).CopyToMemory(nil)
	//    require.NoError(t, err)
	//
	//    memnode := copied.(*state.MemoryNode)
	//    for i := range memnode.Keypaths() {
	//        require.Equal(t, expectedKeypaths[i], memnode.Keypaths()[i])
	//    }
	//    return nil
	//})
	//require.NoError(t, err)
}

func TestMemoryNode_CopyToMemory_AtKeypath(t *testing.T) {
	node := state.NewMemoryNode()
	err := node.Set(nil, nil, fixture1.input)
	require.NoError(t, err)

	copied, err := node.NodeAt(state.Keypath("asdf"), nil).CopyToMemory(nil, nil)
	require.NoError(t, err)

	expected := filterFixtureOutputsWithPrefix(state.Keypath("asdf"), fixture1.output...)

	memnode := copied.(*state.MemoryNode)
	for i, kp := range memnode.Keypaths() {
		require.Equal(t, expected[i].keypath, kp)
		require.Equal(t, expected[i].nodeType, memnode.NodeTypes()[string(kp)])
		if memnode.NodeTypes()[string(kp)] == state.NodeTypeSlice || memnode.NodeTypes()[string(kp)] == state.NodeTypeMap {
			require.Equal(t, uint64(len(expected[i].value.([]interface{}))), memnode.ContentLengths()[string(kp)])
		}
	}
	require.NoError(t, err)
}
