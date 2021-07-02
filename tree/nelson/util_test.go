package nelson_test

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"redwood.dev/internal/testutils"
	"redwood.dev/state"
	"redwood.dev/tree/nelson"
	"redwood.dev/types"
)

func TestFirstNonFrameNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		fixture            interface{}
		keypath            state.Keypath
		maxDepth           uint64
		expected           interface{}
		expectedErrorCause error
	}{
		{
			name: "not a frame, contains frames",
			fixture: M{
				"asdf": M{
					"Content-Type":   "some content type",
					"Content-Length": int64(111),
					"value": M{
						"Content-Type": "foo bar",
						"value": M{
							"value": M{
								"Content-Type": "hi",
							},
						},
					},
				},
			},
			keypath:  state.Keypath(nil),
			maxDepth: 10,
			expected: M{
				"asdf": M{
					"Content-Type":   "some content type",
					"Content-Length": int64(111),
					"value": M{
						"Content-Type": "foo bar",
						"value": M{
							"value": M{
								"Content-Type": "hi",
							},
						},
					},
				},
			},
		},
		{
			name: "frame inside of non-frame, contains frames",
			fixture: M{
				"asdf": M{
					"Content-Type":   "some content type",
					"Content-Length": int64(111),
					"value": M{
						"Content-Type": "foo bar",
						"value": M{
							"value": M{
								"Content-Type": "hi",
							},
						},
					},
				},
			},
			keypath:  state.Keypath("asdf"),
			maxDepth: 10,
			expected: M{
				"Content-Type": "hi",
			},
		},
		{
			name: "frame inside of frame",
			fixture: M{
				"asdf": M{
					"Content-Type":   "some content type",
					"Content-Length": int64(111),
					"value": M{
						"Content-Type": "foo bar",
						"value": M{
							"value": M{
								"Content-Type": "hi",
							},
						},
					},
				},
			},
			keypath:  state.Keypath("asdf/value"),
			maxDepth: 10,
			expected: M{
				"Content-Type": "hi",
			},
		},
		{
			name:     "empty node",
			fixture:  nil,
			keypath:  state.Keypath(nil),
			maxDepth: 10,
			expected: nil,
		},
		{
			name: "insufficient maxDepth",
			fixture: M{
				"asdf": M{
					"Content-Type":   "some content type",
					"Content-Length": int64(111),
					"value": M{
						"Content-Type": "foo bar",
						"value": M{
							"value": M{
								"Content-Type": "hi",
							},
						},
					},
				},
			},
			keypath:            state.Keypath("asdf"),
			maxDepth:           2,
			expected:           nil,
			expectedErrorCause: types.Err404,
		},
	}

	// MemoryNode tests
	for _, test := range tests {
		test := test

		t.Run(test.name+" (MemoryNode)", func(t *testing.T) {
			t.Parallel()

			root := state.NewMemoryNodeWithValue(test.fixture)

			node, err := nelson.FirstNonFrameNode(root.NodeAt(test.keypath, nil), test.maxDepth)
			if test.expectedErrorCause != nil {
				require.Equal(t, test.expectedErrorCause, errors.Cause(err))
				require.Nil(t, node)
			} else {
				value, exists, err := node.Value(nil, nil)
				require.NoError(t, err)
				require.True(t, exists)
				require.Equal(t, test.expected, value)
			}
		})
	}

	// DBNode tests
	for _, test := range tests {
		test := test

		t.Run(test.name+" (DBNode)", func(t *testing.T) {
			t.Parallel()

			db := testutils.SetupVersionedDBTreeWithValue(t, nil, test.fixture)
			defer db.DeleteDB()

			root := db.StateAtVersion(nil, false)
			defer root.Close()

			node, err := nelson.FirstNonFrameNode(root.NodeAt(test.keypath, nil), test.maxDepth)
			if test.expectedErrorCause != nil {
				require.Equal(t, test.expectedErrorCause, errors.Cause(err))
				require.Nil(t, node)
			} else {
				value, exists, err := node.Value(nil, nil)
				require.NoError(t, err)
				require.True(t, exists)
				require.Equal(t, test.expected, value)
			}
		})
	}
}
