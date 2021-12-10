package pb_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

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

	fmt.Println(p.String())
	bs, err := p.MarshalJSON()
	require.NoError(t, err)
	fmt.Println(string(bs))

	var p2 pb.Patch
	err = p2.UnmarshalJSON(bs)
	require.NoError(t, err)
	fmt.Printf("%+v\n", p2)
}
