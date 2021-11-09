package state_test

import (
	"bytes"
	"fmt"
	"sort"

	"redwood.dev/state"
)

type fixture struct {
	input  interface{}
	output []fixtureOutput
}

type fixtureOutput struct {
	keypath  state.Keypath
	nodeType state.NodeType
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
		{state.Keypath(nil), state.NodeTypeMap, testVal1},
		{state.Keypath("asdf"), state.NodeTypeSlice, S{"1234", float64(987.2), uint64(333)}},
		{state.Keypath("asdf").Push(state.EncodeSliceIndex(0)), state.NodeTypeValue, "1234"},
		{state.Keypath("asdf").Push(state.EncodeSliceIndex(1)), state.NodeTypeValue, float64(987.2)},
		{state.Keypath("asdf").Push(state.EncodeSliceIndex(2)), state.NodeTypeValue, uint64(333)},
		{state.Keypath("flo"), state.NodeTypeValue, float64(321)},
		{state.Keypath("flox"), state.NodeTypeSlice, S{uint64(65), M{"yup": "yes", "hey": uint64(321)}, "jkjkjkj"}},
		{state.Keypath("flox").Push(state.EncodeSliceIndex(0)), state.NodeTypeValue, uint64(65)},
		{state.Keypath("flox").Push(state.EncodeSliceIndex(1)), state.NodeTypeMap, M{"yup": "yes", "hey": uint64(321)}},
		{state.Keypath("flox").Push(state.EncodeSliceIndex(1)).Push(state.Keypath("hey")), state.NodeTypeValue, uint64(321)},
		{state.Keypath("flox").Push(state.EncodeSliceIndex(1)).Push(state.Keypath("yup")), state.NodeTypeValue, "yes"},
		{state.Keypath("flox").Push(state.EncodeSliceIndex(2)), state.NodeTypeValue, "jkjkjkj"},
		{state.Keypath("floxxx"), state.NodeTypeValue, "asdf123"},
		{state.Keypath("hello"), state.NodeTypeMap, M{"xyzzy": uint64(33)}},
		{state.Keypath("hello/xyzzy"), state.NodeTypeValue, uint64(33)},
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
		{state.Keypath(nil), state.NodeTypeMap, testVal2},
		{state.Keypath("eee"), state.NodeTypeSlice, S{M{"qqqq": S{"fjfjjfjf", uint64(123321)}}}},
		{state.Keypath("eee").Push(state.EncodeSliceIndex(0)), state.NodeTypeMap, M{"qqqq": S{"fjfjjfjf", uint64(123321)}}},
		{state.Keypath("eee").Push(state.EncodeSliceIndex(0)).Push(state.Keypath("qqqq")), state.NodeTypeSlice, S{"fjfjjfjf", uint64(123321)}},
		{state.Keypath("eee").Push(state.EncodeSliceIndex(0)).Push(state.Keypath("qqqq")).Push(state.EncodeSliceIndex(0)), state.NodeTypeValue, "fjfjjfjf"},
		{state.Keypath("eee").Push(state.EncodeSliceIndex(0)).Push(state.Keypath("qqqq")).Push(state.EncodeSliceIndex(1)), state.NodeTypeValue, uint64(123321)},
		{state.Keypath("ooooooo"), state.NodeTypeValue, uint64(332211)},
	}
	fixture2 = fixture{testVal2, testVal2Output}

	testVal3 = S{
		uint64(8383),
		M{"9999": "hi", "vvvv": "yeah"},
		float64(321.23),
		"hello",
	}
	testVal3Output = []fixtureOutput{
		{state.Keypath(nil), state.NodeTypeSlice, testVal3},
		{state.Keypath(state.EncodeSliceIndex(0)), state.NodeTypeValue, uint64(8383)},
		{state.Keypath(state.EncodeSliceIndex(1)), state.NodeTypeMap, M{"9999": "hi", "vvvv": "yeah"}},
		{state.Keypath(state.EncodeSliceIndex(1)).Push(state.Keypath("9999")), state.NodeTypeValue, "hi"},
		{state.Keypath(state.EncodeSliceIndex(1)).Push(state.Keypath("vvvv")), state.NodeTypeValue, "yeah"},
		{state.Keypath(state.EncodeSliceIndex(2)), state.NodeTypeValue, float64(321.23)},
		{state.Keypath(state.EncodeSliceIndex(3)), state.NodeTypeValue, "hello"},
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
	testVal5Output = []fixtureOutput{{state.Keypath(nil), state.NodeTypeValue, testVal5}}
	fixture5       = fixture{testVal5, testVal5Output}

	testVal6       = "asdfasdf"
	testVal6Output = []fixtureOutput{{state.Keypath(nil), state.NodeTypeValue, testVal6}}
	fixture6       = fixture{testVal6, testVal6Output}

	testVal7       = true
	testVal7Output = []fixtureOutput{{state.Keypath(nil), state.NodeTypeValue, testVal7}}
	fixture7       = fixture{testVal7, testVal7Output}

	testVal8       = S{}
	testVal8Output = []fixtureOutput{{state.Keypath(nil), state.NodeTypeSlice, testVal8}}
	fixture8       = fixture{testVal8, testVal8Output}

	fixtures = []fixture{
		fixture1,
		fixture5,
		fixture6,
		fixture7,
		fixture8,
	}
)

func countNodesOfType(nodeType state.NodeType, outs []fixtureOutput) int {
	i := 0
	for _, out := range outs {
		if out.nodeType == nodeType {
			i++
		}
	}
	return i
}

func combineFixtureOutputs(keypathPrefix state.Keypath, fixtures ...fixture) []fixtureOutput {
	outs := []fixtureOutput{}
	for _, f := range fixtures {
		for _, out := range f.output {
			outs = append(outs, fixtureOutput{keypath: keypathPrefix.Push(out.keypath), nodeType: out.nodeType, value: out.value})
		}
	}
	sort.Slice(outs, func(i, j int) bool { return bytes.Compare(outs[i].keypath, outs[j].keypath) < 0 })
	return outs
}

func prefixFixtureOutputs(keypathPrefix state.Keypath, outs []fixtureOutput) []fixtureOutput {
	newOuts := []fixtureOutput{}
	for _, out := range outs {
		newOuts = append(newOuts, fixtureOutput{keypath: keypathPrefix.Push(out.keypath), nodeType: out.nodeType, value: out.value})
	}
	sort.Slice(newOuts, func(i, j int) bool { return bytes.Compare(newOuts[i].keypath, newOuts[j].keypath) < 0 })
	return newOuts
}

func makeSetKeypathFixtureOutputs(setKeypath state.Keypath) []fixtureOutput {
	if setKeypath.NumParts() == 0 {
		return nil
	}
	var current state.Keypath
	var outs []fixtureOutput
	setKeypathParts := append([]state.Keypath{nil}, setKeypath.Parts()...)
	setKeypathParts = setKeypathParts[:len(setKeypathParts)-1] // Remove the last item -- it's added by the test fixture
	for _, part := range setKeypathParts {
		current = current.Push(part)
		outs = append(outs, fixtureOutput{
			keypath:  current,
			nodeType: state.NodeTypeMap,
		})
	}
	return outs
}

func filterFixtureOutputsWithPrefix(prefix state.Keypath, outs ...fixtureOutput) []fixtureOutput {
	var newOuts []fixtureOutput
	for _, out := range outs {
		if out.keypath.StartsWith(prefix) {
			newOuts = append(newOuts, out)
		}
	}
	return newOuts
}

func filterFixtureOutputsToDirectDescendantsOf(prefix state.Keypath, outs ...fixtureOutput) []fixtureOutput {
	var newOuts []fixtureOutput
	for _, out := range outs {
		if out.keypath.StartsWith(prefix) && out.keypath.NumParts() == prefix.NumParts()+1 {
			newOuts = append(newOuts, out)
		}
	}
	return newOuts
}

func removeFixtureOutputsWithPrefix(prefix state.Keypath, outs ...fixtureOutput) []fixtureOutput {
	var newOuts []fixtureOutput
	for _, out := range outs {
		if !out.keypath.StartsWith(prefix) {
			newOuts = append(newOuts, out)
		}
	}
	return newOuts
}

func renumberSliceFixtureOutputsWithPrefix(prefix state.Keypath, startIndex uint64, delta int64, outs ...fixtureOutput) []fixtureOutput {
	var newOuts []fixtureOutput
	for _, out := range outs {
		if !out.keypath.StartsWith(prefix) {
			newOuts = append(newOuts, out)
			continue
		}
		tail := out.keypath.RelativeTo(prefix)
		if len(tail) == 0 {
			newOuts = append(newOuts, out)
			continue
		}
		oldIdxKey, rest := tail.Shift()
		oldIdx := state.DecodeSliceIndex(oldIdxKey)
		if oldIdx < startIndex {
			newOuts = append(newOuts, out)
			continue
		}

		newIdx := uint64(int64(oldIdx) + delta)
		newIdxKey := state.EncodeSliceIndex(newIdx)
		newKeypath := prefix.Push(newIdxKey).Push(rest)
		newOuts = append(newOuts, fixtureOutput{
			keypath:  newKeypath,
			nodeType: out.nodeType,
			value:    out.value,
		})
	}
	return newOuts
}

func removeFixtureOutputPrefixes(prefix state.Keypath, outs ...fixtureOutput) []fixtureOutput {
	var newOuts []fixtureOutput
	for _, out := range outs {
		if !out.keypath.StartsWith(prefix) {
			panic("bad test: fixtureOutput does not have prefix")
		} else {
			if len(prefix) == 0 {
				newOuts = append(newOuts, out)
			} else {
				if len(prefix) == len(out.keypath) {
					newOuts = append(newOuts, fixtureOutput{keypath: state.Keypath(nil), nodeType: out.nodeType, value: out.value})
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
