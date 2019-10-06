package redwood_test

import (
	"fmt"
	"testing"

	rw "github.com/brynbellomy/redwood"
)

func TestPatchParser(t *testing.T) {
	str := `.shrugisland.talk0.validator = {
        "type":"stack",
        "children":[
            {"type": "intrinsics"},
            {"type": "permissions"}
        ]
    }`

	p, err := rw.ParsePatch(str)
	if err != nil {
		t.Fatal(err)
	}
}
