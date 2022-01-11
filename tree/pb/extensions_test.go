package pb_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/tree/pb"
)

type M map[string]interface{}

func TestPatchString(t *testing.T) {
	valueJSON, err := json.Marshal(M{
		"yeet":  M{"blah": "yes"},
		"hello": "hi",
	})
	require.NoError(t, err)

	p := pb.Patch{
		Keypath:   state.Keypath("foo/bar/baz"),
		Range:     nil,
		ValueJSON: valueJSON,
	}

	bs, err := p.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, `".foo.bar.baz = {\"hello\":\"hi\",\"yeet\":{\"blah\":\"yes\"}}"`, string(bs))
}

func TestPatch_Unmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		patchStr    string
		expected    pb.Patch
		expectedErr error
	}{
		{`.text.value[0:0] = "a"`, pb.Patch{state.Keypath("text/value"), &state.Range{0, 0, false}, []byte(`"a"`)}, nil},
		{`.text.value[-0:-0] = "a"`, pb.Patch{state.Keypath("text/value"), &state.Range{0, 0, true}, []byte(`"a"`)}, nil},
		{`.text.value[-0:0] = "a"`, pb.Patch{state.Keypath("text/value"), &state.Range{0, 0, true}, []byte(`"a"`)}, nil},
		{`.text.value[0:3] = "a"`, pb.Patch{state.Keypath("text/value"), &state.Range{0, 3, false}, []byte(`"a"`)}, nil},
		{`.text.value[2:3] = "a"`, pb.Patch{state.Keypath("text/value"), &state.Range{2, 3, false}, []byte(`"a"`)}, nil},
		{`.text.value[3:1] = "a"`, pb.Patch{state.Keypath("text/value"), &state.Range{3, 1, false}, []byte(`"a"`)}, nil},
		{`[1:5] = "a"`, pb.Patch{nil, &state.Range{1, 5, false}, []byte(`"a"`)}, nil},
		{`.[1:5] = "a"`, pb.Patch{nil, &state.Range{1, 5, false}, []byte(`"a"`)}, nil},
		{`.text.value = "a"`, pb.Patch{state.Keypath("text/value"), nil, []byte(`"a"`)}, nil},
		{`.text.value = {"foo": "bar"}`, pb.Patch{state.Keypath("text/value"), nil, []byte(`{"foo": "bar"}`)}, nil},
		{`. = {"foo": "bar"}`, pb.Patch{nil, nil, []byte(`{"foo": "bar"}`)}, nil},
		{`= {"foo": "bar"}`, pb.Patch{nil, nil, []byte(`{"foo": "bar"}`)}, nil},
		{` = {"foo": "bar"}`, pb.Patch{nil, nil, []byte(`{"foo": "bar"}`)}, nil},
		{`.[3:5] = {"foo": "bar"}`, pb.Patch{nil, &state.Range{3, 5, false}, []byte(`{"foo": "bar"}`)}, nil},
		{`[3:5] = {"foo": "bar"}`, pb.Patch{nil, &state.Range{3, 5, false}, []byte(`{"foo": "bar"}`)}, nil},
		{`.text.value[3:5] = {"foo": "bar"}`, pb.Patch{state.Keypath("text/value"), &state.Range{3, 5, false}, []byte(`{"foo": "bar"}`)}, nil},
		{`.text.value[3:5] = asdfasdf`, pb.Patch{state.Keypath("text/value"), &state.Range{3, 5, false}, []byte(`asdfasdf`)}, nil},
		{`.text.value[3] = "a"`, pb.Patch{state.Keypath("text/value").PushIndex(3), nil, []byte(`"a"`)}, nil},
		{`.text.value["foo"] = "a"`, pb.Patch{state.Keypath("text/value/foo"), nil, []byte(`"a"`)}, nil},
		{`.text.value["foo"].bar = "a"`, pb.Patch{state.Keypath("text/value/foo/bar"), nil, []byte(`"a"`)}, nil},
		{`.text.value["foo.bar"] = "a"`, pb.Patch{state.Keypath("text/value/foo.bar"), nil, []byte(`"a"`)}, nil},
		{`.text.value["foo.bar"].baz = "a"`, pb.Patch{state.Keypath("text/value/foo.bar/baz"), nil, []byte(`"a"`)}, nil},
		{`.["foo"].bar = "a"`, pb.Patch{state.Keypath("foo/bar"), nil, []byte(`"a"`)}, nil},
		{`["foo"].bar = "a"`, pb.Patch{state.Keypath("foo/bar"), nil, []byte(`"a"`)}, nil},
		{`["foo.bar"].baz = "a"`, pb.Patch{state.Keypath("foo.bar/baz"), nil, []byte(`"a"`)}, nil},
		{`.text.value[-3] = "a"`, pb.Patch{}, state.ErrBadKeypath},
		{`.text.value[] = "a"`, pb.Patch{}, state.ErrBadKeypath},
		{`.text.value[] = `, pb.Patch{}, state.ErrBadKeypath},
		{`.text.value[foo] = "a"`, pb.Patch{}, state.ErrBadKeypath},
		{`text.value = "a"`, pb.Patch{}, state.ErrBadKeypath},
	}

	for _, test := range tests {
		test := test
		t.Run(test.patchStr, func(t *testing.T) {
			t.Parallel()

			var patch pb.Patch
			err := patch.UnmarshalText([]byte(test.patchStr))
			if test.expectedErr != nil {
				assert.Equal(t, pb.Patch{}, patch)
				assert.Equal(t, test.expectedErr, errors.Cause(err))
			} else {
				assert.Equal(t, test.expected, patch)
				assert.NoError(t, err)
			}
		})
	}
}
