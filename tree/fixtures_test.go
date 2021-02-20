package tree_test

import (
	"bytes"
	"fmt"
	"sort"

	"redwood.dev/tree"
)

type fixture struct {
	input  interface{}
	output []fixtureOutput
}

type fixtureOutput struct {
	keypath  tree.Keypath
	nodeType tree.NodeType
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
		{tree.Keypath(nil), tree.NodeTypeMap, testVal1},
		{tree.Keypath("asdf"), tree.NodeTypeSlice, S{"1234", float64(987.2), uint64(333)}},
		{tree.Keypath("asdf").Push(tree.EncodeSliceIndex(0)), tree.NodeTypeValue, "1234"},
		{tree.Keypath("asdf").Push(tree.EncodeSliceIndex(1)), tree.NodeTypeValue, float64(987.2)},
		{tree.Keypath("asdf").Push(tree.EncodeSliceIndex(2)), tree.NodeTypeValue, uint64(333)},
		{tree.Keypath("flo"), tree.NodeTypeValue, float64(321)},
		{tree.Keypath("flox"), tree.NodeTypeSlice, S{uint64(65), M{"yup": "yes", "hey": uint64(321)}, "jkjkjkj"}},
		{tree.Keypath("flox").Push(tree.EncodeSliceIndex(0)), tree.NodeTypeValue, uint64(65)},
		{tree.Keypath("flox").Push(tree.EncodeSliceIndex(1)), tree.NodeTypeMap, M{"yup": "yes", "hey": uint64(321)}},
		{tree.Keypath("flox").Push(tree.EncodeSliceIndex(1)).Push(tree.Keypath("hey")), tree.NodeTypeValue, uint64(321)},
		{tree.Keypath("flox").Push(tree.EncodeSliceIndex(1)).Push(tree.Keypath("yup")), tree.NodeTypeValue, "yes"},
		{tree.Keypath("flox").Push(tree.EncodeSliceIndex(2)), tree.NodeTypeValue, "jkjkjkj"},
		{tree.Keypath("floxxx"), tree.NodeTypeValue, "asdf123"},
		{tree.Keypath("hello"), tree.NodeTypeMap, M{"xyzzy": uint64(33)}},
		{tree.Keypath("hello/xyzzy"), tree.NodeTypeValue, uint64(33)},
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
		{tree.Keypath(nil), tree.NodeTypeMap, testVal2},
		{tree.Keypath("eee"), tree.NodeTypeSlice, S{M{"qqqq": S{"fjfjjfjf", uint64(123321)}}}},
		{tree.Keypath("eee").Push(tree.EncodeSliceIndex(0)), tree.NodeTypeMap, M{"qqqq": S{"fjfjjfjf", uint64(123321)}}},
		{tree.Keypath("eee").Push(tree.EncodeSliceIndex(0)).Push(tree.Keypath("qqqq")), tree.NodeTypeSlice, S{"fjfjjfjf", uint64(123321)}},
		{tree.Keypath("eee").Push(tree.EncodeSliceIndex(0)).Push(tree.Keypath("qqqq")).Push(tree.EncodeSliceIndex(0)), tree.NodeTypeValue, "fjfjjfjf"},
		{tree.Keypath("eee").Push(tree.EncodeSliceIndex(0)).Push(tree.Keypath("qqqq")).Push(tree.EncodeSliceIndex(1)), tree.NodeTypeValue, uint64(123321)},
		{tree.Keypath("ooooooo"), tree.NodeTypeValue, uint64(332211)},
	}
	fixture2 = fixture{testVal2, testVal2Output}

	testVal3 = S{
		uint64(8383),
		M{"9999": "hi", "vvvv": "yeah"},
		float64(321.23),
		"hello",
	}
	testVal3Output = []fixtureOutput{
		{tree.Keypath(nil), tree.NodeTypeSlice, testVal3},
		{tree.Keypath(tree.EncodeSliceIndex(0)), tree.NodeTypeValue, uint64(8383)},
		{tree.Keypath(tree.EncodeSliceIndex(1)), tree.NodeTypeMap, M{"9999": "hi", "vvvv": "yeah"}},
		{tree.Keypath(tree.EncodeSliceIndex(1)).Push(tree.Keypath("9999")), tree.NodeTypeValue, "hi"},
		{tree.Keypath(tree.EncodeSliceIndex(1)).Push(tree.Keypath("vvvv")), tree.NodeTypeValue, "yeah"},
		{tree.Keypath(tree.EncodeSliceIndex(2)), tree.NodeTypeValue, float64(321.23)},
		{tree.Keypath(tree.EncodeSliceIndex(3)), tree.NodeTypeValue, "hello"},
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
	testVal5Output = []fixtureOutput{{tree.Keypath(nil), tree.NodeTypeValue, testVal5}}
	fixture5       = fixture{testVal5, testVal5Output}

	testVal6       = "asdfasdf"
	testVal6Output = []fixtureOutput{{tree.Keypath(nil), tree.NodeTypeValue, testVal6}}
	fixture6       = fixture{testVal6, testVal6Output}

	testVal7       = true
	testVal7Output = []fixtureOutput{{tree.Keypath(nil), tree.NodeTypeValue, testVal7}}
	fixture7       = fixture{testVal7, testVal7Output}

	testVal8       = S{}
	testVal8Output = []fixtureOutput{{tree.Keypath(nil), tree.NodeTypeSlice, testVal8}}
	fixture8       = fixture{testVal8, testVal8Output}

	fixtures = []fixture{
		fixture1,
		fixture5,
		fixture6,
		fixture7,
		fixture8,
	}
)

func countNodesOfType(nodeType tree.NodeType, outs []fixtureOutput) int {
	i := 0
	for _, out := range outs {
		if out.nodeType == nodeType {
			i++
		}
	}
	return i
}

func combineFixtureOutputs(keypathPrefix tree.Keypath, fixtures ...fixture) []fixtureOutput {
	outs := []fixtureOutput{}
	for _, f := range fixtures {
		for _, out := range f.output {
			outs = append(outs, fixtureOutput{keypath: keypathPrefix.Push(out.keypath), nodeType: out.nodeType, value: out.value})
		}
	}
	sort.Slice(outs, func(i, j int) bool { return bytes.Compare(outs[i].keypath, outs[j].keypath) < 0 })
	return outs
}

func prefixFixtureOutputs(keypathPrefix tree.Keypath, outs []fixtureOutput) []fixtureOutput {
	newOuts := []fixtureOutput{}
	for _, out := range outs {
		newOuts = append(newOuts, fixtureOutput{keypath: keypathPrefix.Push(out.keypath), nodeType: out.nodeType, value: out.value})
	}
	sort.Slice(newOuts, func(i, j int) bool { return bytes.Compare(newOuts[i].keypath, newOuts[j].keypath) < 0 })
	return newOuts
}

func makeSetKeypathFixtureOutputs(setKeypath tree.Keypath) []fixtureOutput {
	if setKeypath.NumParts() == 0 {
		return nil
	}
	var current tree.Keypath
	var outs []fixtureOutput
	setKeypathParts := append([]tree.Keypath{nil}, setKeypath.Parts()...)
	setKeypathParts = setKeypathParts[:len(setKeypathParts)-1] // Remove the last item -- it's added by the test fixture
	for _, part := range setKeypathParts {
		current = current.Push(part)
		outs = append(outs, fixtureOutput{
			keypath:  current,
			nodeType: tree.NodeTypeMap,
		})
	}
	return outs
}

func filterFixtureOutputsWithPrefix(prefix tree.Keypath, outs ...fixtureOutput) []fixtureOutput {
	var newOuts []fixtureOutput
	for _, out := range outs {
		if out.keypath.StartsWith(prefix) {
			newOuts = append(newOuts, out)
		}
	}
	return newOuts
}

func filterFixtureOutputsToDirectDescendantsOf(prefix tree.Keypath, outs ...fixtureOutput) []fixtureOutput {
	var newOuts []fixtureOutput
	for _, out := range outs {
		if out.keypath.StartsWith(prefix) && out.keypath.NumParts() == prefix.NumParts()+1 {
			newOuts = append(newOuts, out)
		}
	}
	return newOuts
}

func removeFixtureOutputsWithPrefix(prefix tree.Keypath, outs ...fixtureOutput) []fixtureOutput {
	var newOuts []fixtureOutput
	for _, out := range outs {
		if !out.keypath.StartsWith(prefix) {
			newOuts = append(newOuts, out)
		}
	}
	return newOuts
}

func removeFixtureOutputPrefixes(prefix tree.Keypath, outs ...fixtureOutput) []fixtureOutput {
	var newOuts []fixtureOutput
	for _, out := range outs {
		if !out.keypath.StartsWith(prefix) {
			panic("bad test: fixtureOutput does not have prefix")
		} else {
			if len(prefix) == 0 {
				newOuts = append(newOuts, out)
			} else {
				if len(prefix) == len(out.keypath) {
					newOuts = append(newOuts, fixtureOutput{keypath: tree.Keypath(nil), nodeType: out.nodeType, value: out.value})
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

func reverseFixtureOutputs(outs ...fixtureOutput) []fixtureOutput {
	var newOuts []fixtureOutput
	for i := len(outs) - 1; i >= 0; i-- {
		newOuts = append(newOuts, outs[i])
	}
	return newOuts
}

func debugPrintFixtureOutputs(outs []fixtureOutput) {
	for _, out := range outs {
		fmt.Println(out.keypath)
	}
}
