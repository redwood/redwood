package tree

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

type fixture struct {
	input  interface{}
	output []fixtureOutput
}

type fixtureOutput struct {
	keypath  Keypath
	nodeType NodeType
	value    interface{}
}

type M = map[string]interface{}
type S = []interface{}

var (
	testVal1 = M{
		"asdf": S{"1234", float64(987.2), uint64(333)},
		"flo":  float64(321),
		"flox": S{
			uint64(65),
			M{"yup": "yes", "hey": uint64(321)},
			"jkjkjkj",
		},
		"floxxx": "asdf123",
		"hello": M{
			"xyzzy": uint64(33),
		},
	}
	testVal1Output = []fixtureOutput{
		{Keypath(nil), NodeTypeMap, testVal1},
		{Keypath("asdf"), NodeTypeSlice, S{"1234", float64(987.2), uint64(333)}},
		{Keypath("asdf").Push(EncodeSliceIndex(0)), NodeTypeValue, "1234"},
		{Keypath("asdf").Push(EncodeSliceIndex(1)), NodeTypeValue, float64(987.2)},
		{Keypath("asdf").Push(EncodeSliceIndex(2)), NodeTypeValue, uint64(333)},
		{Keypath("flo"), NodeTypeValue, float64(321)},
		{Keypath("flox"), NodeTypeSlice, S{uint64(65), M{"yup": "yes", "hey": uint64(321)}, "jkjkjkj"}},
		{Keypath("flox").Push(EncodeSliceIndex(0)), NodeTypeValue, uint64(65)},
		{Keypath("flox").Push(EncodeSliceIndex(1)), NodeTypeMap, M{"yup": "yes", "hey": uint64(321)}},
		{Keypath("flox").Push(EncodeSliceIndex(1)).Push(Keypath("hey")), NodeTypeValue, uint64(321)},
		{Keypath("flox").Push(EncodeSliceIndex(1)).Push(Keypath("yup")), NodeTypeValue, "yes"},
		{Keypath("flox").Push(EncodeSliceIndex(2)), NodeTypeValue, "jkjkjkj"},
		{Keypath("floxxx"), NodeTypeValue, "asdf123"},
		{Keypath("hello"), NodeTypeMap, M{"xyzzy": uint64(33)}},
		{Keypath("hello/xyzzy"), NodeTypeValue, uint64(33)},
	}
	fixture1 = fixture{testVal1, testVal1Output}

	testVal2 = M{
		"ooooooo": uint64(332211),
		"eee": S{
			M{
				"qqqq": S{"fjfjjfjf", uint64(123321)},
			},
		},
	}
	testVal2Output = []fixtureOutput{
		{Keypath(nil), NodeTypeMap, testVal2},
		{Keypath("eee"), NodeTypeSlice, S{M{"qqqq": S{"fjfjjfjf", uint64(123321)}}}},
		{Keypath("eee").Push(EncodeSliceIndex(0)), NodeTypeMap, M{"qqqq": S{"fjfjjfjf", uint64(123321)}}},
		{Keypath("eee").Push(EncodeSliceIndex(0)).Push(Keypath("qqqq")), NodeTypeSlice, S{"fjfjjfjf", uint64(123321)}},
		{Keypath("eee").Push(EncodeSliceIndex(0)).Push(Keypath("qqqq")).Push(EncodeSliceIndex(0)), NodeTypeValue, "fjfjjfjf"},
		{Keypath("eee").Push(EncodeSliceIndex(0)).Push(Keypath("qqqq")).Push(EncodeSliceIndex(1)), NodeTypeValue, uint64(123321)},
		{Keypath("ooooooo"), NodeTypeValue, uint64(332211)},
	}
	fixture2 = fixture{testVal2, testVal2Output}

	testVal3 = S{
		uint64(8383),
		M{"9999": "hi", "vvvv": "yeah"},
		float64(321.23),
		"hello",
	}
	testVal3Output = []fixtureOutput{
		{Keypath(nil), NodeTypeSlice, testVal3},
		{Keypath(EncodeSliceIndex(0)), NodeTypeValue, uint64(8383)},
		{Keypath(EncodeSliceIndex(1)), NodeTypeMap, M{"9999": "hi", "vvvv": "yeah"}},
		{Keypath(EncodeSliceIndex(1)).Push(Keypath("9999")), NodeTypeValue, "hi"},
		{Keypath(EncodeSliceIndex(1)).Push(Keypath("vvvv")), NodeTypeValue, "yeah"},
		{Keypath(EncodeSliceIndex(2)), NodeTypeValue, float64(321.23)},
		{Keypath(EncodeSliceIndex(3)), NodeTypeValue, "hello"},
	}
	fixture3 = fixture{testVal3, testVal3Output}

	testVal4 = M{
		"lkjlkj": M{
			"aaa": float64(33),
			"bbb": S{true, uint64(234), "fjeijfiejf", float64(321)},
		},
		"zzzzz": S{
			M{"eee": "fff", "ggg": false},
			M{"hhh": true},
			true,
		},
	}

	testVal5       = float64(123.443)
	testVal5Output = []fixtureOutput{{Keypath(nil), NodeTypeValue, testVal5}}
	fixture5       = fixture{testVal5, testVal5Output}

	testVal6       = "asdfasdf"
	testVal6Output = []fixtureOutput{{Keypath(nil), NodeTypeValue, testVal6}}
	fixture6       = fixture{testVal6, testVal6Output}

	testVal7       = true
	testVal7Output = []fixtureOutput{{Keypath(nil), NodeTypeValue, testVal7}}
	fixture7       = fixture{testVal7, testVal7Output}

	testVal8       = S{}
	testVal8Output = []fixtureOutput{{Keypath(nil), NodeTypeSlice, testVal8}}
	fixture8       = fixture{testVal8, testVal8Output}

	fixtures = []fixture{
		fixture1,
		fixture5,
		fixture6,
		fixture7,
		fixture8,
	}
)

func countNodesOfType(nodeType NodeType, outs ...fixtureOutput) int {
	i := 0
	for _, out := range outs {
		if out.nodeType == nodeType {
			i++
		}
	}
	return i
}

func combineFixtureOutputs(keypathPrefix Keypath, fixtures ...fixture) []fixtureOutput {
	outs := []fixtureOutput{}
	for _, f := range fixtures {
		for _, out := range f.output {
			outs = append(outs, fixtureOutput{keypath: keypathPrefix.Push(out.keypath), nodeType: out.nodeType, value: out.value})
		}
	}
	sort.Slice(outs, func(i, j int) bool { return bytes.Compare(outs[i].keypath, outs[j].keypath) < 0 })
	return outs
}

func makeAtKeypathFixtureOutputs(atKeypath Keypath) []fixtureOutput {
	if atKeypath.NumParts() == 0 {
		return nil
	}
	var current Keypath
	var outs []fixtureOutput
	atKeypathParts := append([]Keypath{nil}, atKeypath.Parts()...)
	atKeypathParts = atKeypathParts[:len(atKeypathParts)-1] // Remove the last item -- it's added by the test fixture
	for _, part := range atKeypathParts {
		current = current.Push(part)
		outs = append(outs, fixtureOutput{
			keypath:  current,
			nodeType: NodeTypeMap,
		})
	}
	return outs
}

func takeFixtureOutputsWithPrefix(prefix Keypath, outs ...fixtureOutput) []fixtureOutput {
	var newOuts []fixtureOutput
	for _, out := range outs {
		if out.keypath.StartsWith(prefix) {
			newOuts = append(newOuts, out)
		}
	}
	return newOuts
}

func removeFixtureOutputsWithPrefix(prefix Keypath, outs ...fixtureOutput) []fixtureOutput {
	var newOuts []fixtureOutput
	for _, out := range outs {
		if !out.keypath.StartsWith(prefix) {
			newOuts = append(newOuts, out)
		}
	}
	return newOuts
}

func removeFixtureOutputPrefixes(prefix Keypath, outs ...fixtureOutput) []fixtureOutput {
	var newOuts []fixtureOutput
	for _, out := range outs {
		if !out.keypath.StartsWith(prefix) {
			panic("bad test: fixtureOutput does not have prefix")
		} else {
			if len(prefix) == 0 {
				newOuts = append(newOuts, out)
			} else {
				if len(prefix) == len(out.keypath) {
					newOuts = append(newOuts, fixtureOutput{keypath: Keypath(nil), nodeType: out.nodeType, value: out.value})
				} else {
					newKeypath := out.keypath.Copy()
					newKeypath = newKeypath[len(prefix)+1:]
					newOuts = append(newOuts, fixtureOutput{keypath: newKeypath, nodeType: out.nodeType, value: out.value})
				}
			}
		}
	}
	return newOuts
}

func mustEncodeGoValue(x interface{}) []byte {
	enc, err := encodeGoValue(x)
	if err != nil {
		panic(err)
	}
	return enc
}

func mustNewDBTree(T *testing.T) *DBTree {
	tree, err := NewDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", rand.Int()))
	require.NoError(T, err)
	return tree
}
