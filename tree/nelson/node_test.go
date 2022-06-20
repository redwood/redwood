package nelson_test

// import (
// 	"testing"

// 	"github.com/stretchr/testify/require"

// 	"redwood.dev/state"
// 	"redwood.dev/tree/nelson"
// )

// func TestFrame_ParentNodeAt(t *testing.T) {
// 	node := state.NewMemoryNodeWithValue(M{"foo": M{}})

// 	frame2 := nelson.Frame{
// 		Node: state.NewMemoryNodeWithValue(M{
// 			"quux": uint64(12345),
// 		}),
// 	}
// 	frame := nelson.Frame{
// 		Node: state.NewMemoryNodeWithValue(M{
// 			"baz": frame2,
// 		}),
// 	}

// 	err := node.Set(state.Keypath("foo/bar"), nil, frame)
// 	require.NoError(t, err)

// 	parent, remainingKeypath := node.ParentNodeFor(state.Keypath("foo/bar/baz/quux"))
// 	require.Equal(t, frame2, parent)
// 	require.Equal(t, state.Keypath("quux"), remainingKeypath)

// 	parent, remainingKeypath = node.ParentNodeFor(state.Keypath("foo/bar/baz"))
// 	require.Equal(t, frame.Node, parent)
// 	require.Equal(t, state.Keypath("quux"), remainingKeypath)
// }
