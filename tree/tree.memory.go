package tree

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/pkg/errors"
)

type MemoryNode struct {
	keypath Keypath
	rng     *Range

	keypaths     []Keypath
	values       map[string]interface{}
	nodeTypes    map[string]NodeType
	sliceLengths map[string]int
	copied       bool
	diff         *Diff
}

func NewMemoryNode() Node {
	values := make(map[string]interface{})
	//if rootNodeType == NodeTypeValue {
	//    values[""] = nil
	//}
	return &MemoryNode{
		//keypaths: []Keypath{Keypath(nil)},
		values:    values,
		nodeTypes: map[string]NodeType{
			//"": rootNodeType,
		},
		sliceLengths: map[string]int{},
		diff:         NewDiff(),
	}
}

func (nt NodeType) String() string {
	switch nt {
	case NodeTypeValue:
		return "Value"
	case NodeTypeMap:
		return "Map"
	case NodeTypeSlice:
		return "Slice"
	default:
		return "Invalid"
	}
}

func (n *MemoryNode) Close() {
}

func (n *MemoryNode) Keypath() Keypath {
	return n.keypath
}

func (n *MemoryNode) DebugContents(keypathPrefix Keypath, rng *[2]uint64) ([]Keypath, []interface{}, map[string]NodeType, error) {
	values := make([]interface{}, len(n.keypaths))
	for i := range n.keypaths {
		values[i] = n.values[string(n.keypaths[i])]
	}
	return n.keypaths, values, n.nodeTypes, nil
}

// CopyToMemory returns a copy of the node at the given keypath.  However, it uses
// copy-on-write under the hood, so that if the node and its children are not
// modified, no actual copying is done.
func (t *MemoryNode) CopyToMemory(keypath Keypath, rng *Range) (Node, error) {
	if rng == nil {
		rng = t.rng
	} else if rng != nil && t.rng != nil {
		panic("unsupported")
	}
	if rng != nil && !rng.Valid() {
		return nil, errors.WithStack(ErrInvalidRange)
	}

	//t.copied = true
	cpy := &MemoryNode{
		keypath:   t.keypath.Push(keypath),
		rng:       rng,
		keypaths:  t.keypaths,
		values:    t.values,
		nodeTypes: t.nodeTypes,
		diff:      t.diff,
		//copied:    true,
	}
	cpy.makeCopy()
	return cpy, nil
}

func (t *MemoryNode) checkCopied() {
	if !t.copied {
		return
	}
	t.makeCopy()
}

func (t *MemoryNode) makeCopy() {
	start := 0
	end := len(t.keypaths)
	if len(t.keypath) > 0 {
		start, end = t.findPrefixRange(t.keypath)
	}
	if start == -1 {
		start = 0
		end = 0
	}

	keypaths := make([]Keypath, end-start)
	values := make(map[string]interface{}, end-start)
	nodeTypes := make(map[string]NodeType, end-start)

	copy(keypaths, t.keypaths[start:end])

	for _, kp := range keypaths {
		values[string(kp)] = t.values[string(kp)]
		nodeTypes[string(kp)] = t.nodeTypes[string(kp)]
	}

	t.keypaths = keypaths
	t.values = values
	t.nodeTypes = nodeTypes
	t.diff = t.diff.Copy()

	t.copied = false
}

func (t *MemoryNode) AtKeypath(keypath Keypath, rng *Range) Node {
	return &MemoryNode{keypath: t.keypath.Push(keypath), rng: rng, keypaths: t.keypaths, values: t.values, nodeTypes: t.nodeTypes, diff: t.diff}
}

func (t *MemoryNode) Exists(keypath Keypath) (bool, error) {
	_, exists := t.values[string(t.keypath.Push(keypath))]
	return exists, nil
}

func (t *MemoryNode) UintValue(keypath Keypath) (uint64, bool, error) {
	absKeypath := t.keypath.Push(keypath)
	v, exists := t.values[string(absKeypath)]
	if !exists {
		return 0, false, nil
	}
	if asUint, isUint := v.(uint64); isUint {
		return asUint, true, nil
	}
	return 0, false, nil
}

func (t *MemoryNode) IntValue(keypath Keypath) (int64, bool, error) {
	absKeypath := t.keypath.Push(keypath)
	v, exists := t.values[string(absKeypath)]
	if !exists {
		return 0, false, nil
	}
	if asInt, isInt := v.(int64); isInt {
		return asInt, true, nil
	}
	return 0, false, nil
}

func (t *MemoryNode) FloatValue(keypath Keypath) (float64, bool, error) {
	absKeypath := t.keypath.Push(keypath)
	v, exists := t.values[string(absKeypath)]
	if !exists {
		return 0, false, nil
	}
	if asFloat, isFloat := v.(float64); isFloat {
		return asFloat, true, nil
	}
	return 0, false, nil
}

func (t *MemoryNode) StringValue(keypath Keypath) (string, bool, error) {
	absKeypath := t.keypath.Push(keypath)
	v, exists := t.values[string(absKeypath)]
	if !exists {
		return "", false, nil
	}
	if asString, isString := v.(string); isString {
		return asString, true, nil
	}
	return "", false, nil
}

func (t *MemoryNode) Value(keypath Keypath, rng *Range) (interface{}, bool, error) {
	if rng == nil {
		rng = t.rng
	} else if rng != nil && t.rng != nil {
		panic("unsupported")
	}
	if rng != nil && !rng.Valid() {
		return nil, false, errors.WithStack(ErrInvalidRange)
	}

	absKeypath := t.keypath.Push(keypath)

	switch t.nodeTypes[string(absKeypath)] {
	case NodeTypeInvalid:
		return nil, false, nil

	case NodeTypeValue:
		if rng != nil {
			return nil, false, ErrRangeOverNonSlice
		}
		val, exists := t.values[string(absKeypath)]
		return val, exists, nil

	case NodeTypeMap:
		if rng != nil {
			return nil, false, ErrRangeOverNonSlice
		}

		m := make(map[string]interface{})

		t.scanNodeTypesWithPrefix(absKeypath, func(kp Keypath, nodeType NodeType) {
			if nodeType == NodeTypeSlice {
				relKp := kp.RelativeTo(absKeypath)
				if len(relKp) != 0 {
					setValueAtKeypath(m, relKp, make([]interface{}, t.sliceLengths[string(kp)]), false)
				}
			}
		})

		t.scanKeypathsWithPrefix(absKeypath, nil, func(kp Keypath, _ int) error {
			relKp := kp.RelativeTo(absKeypath)
			if len(relKp) != 0 {
				setValueAtKeypath(m, relKp, t.values[string(kp)], false)
			}
			return nil
		})
		return m, true, nil

	case NodeTypeSlice:
		s := make([]interface{}, t.sliceLengths[string(absKeypath)])

		t.scanNodeTypesWithPrefix(absKeypath, func(kp Keypath, nodeType NodeType) {
			if nodeType == NodeTypeSlice {
				relKp := kp.RelativeTo(absKeypath)
				if len(relKp) != 0 {
					setValueAtKeypath(s, relKp, make([]interface{}, t.sliceLengths[string(kp)]), false)
				}
			}
		})

		t.scanKeypathsWithPrefix(absKeypath, rng, func(kp Keypath, _ int) error {
			relKp := kp.RelativeTo(absKeypath)
			if len(relKp) != 0 {
				setValueAtKeypath(s, relKp, t.values[string(kp)], false)
			}
			return nil
		})
		return s, true, nil

	default:
		panic("tree.Value(): bad NodeType")
	}
}

func (n *MemoryNode) ContentLength() (int64, error) {
	switch n.nodeTypes[string(n.keypath)] {
	case NodeTypeMap:
		return 0, nil
	case NodeTypeSlice:
		return int64(n.sliceLengths[string(n.keypath)]), nil
	case NodeTypeValue:
		switch v := n.values[string(n.keypath)].(type) {
		case string:
			return int64(len(v)), nil
		default:
			return 0, nil
		}
	default:
		return 0, nil
	}
}

func (t *MemoryNode) Set(keypath Keypath, rng *Range, value interface{}) error {
	// @@TODO: handle range
	if rng != nil {
		panic("unsupported")
	}

	t.checkCopied()

	err := t.Delete(keypath, rng)
	if err != nil {
		return err
	}

	absKeypath := t.keypath.Push(keypath)

	var newKeypaths []Keypath

	// Set value types for intermediate keypaths in case they don't exist
	{
		var isSettingSimpleValue bool
		switch value.(type) {
		case map[string]interface{}, []interface{}:
		default:
			isSettingSimpleValue = true
		}
		if len(absKeypath) != 0 || !isSettingSimpleValue {
			nt := t.nodeTypes[string(Keypath(nil))]
			if nt == NodeTypeValue {
				t.nodeTypes[string(Keypath(nil))] = NodeTypeMap
			} else if nt == NodeTypeInvalid {
				t.nodeTypes[string(Keypath(nil))] = NodeTypeMap
				//newKeypaths = append(newKeypaths, nil)
			}
		}

		parts := absKeypath.Parts()
		if len(parts) != 0 {
			var byteIdx int
			for _, key := range parts[:len(parts)-1] {
				byteIdx += len(key)

				partialKeypath := absKeypath[:byteIdx]
				nt := t.nodeTypes[string(partialKeypath)]
				if nt == NodeTypeValue {
					t.nodeTypes[string(partialKeypath)] = NodeTypeMap
				} else if nt == NodeTypeInvalid {
					t.nodeTypes[string(partialKeypath)] = NodeTypeMap
					newKeypaths = append(newKeypaths, partialKeypath)
				}
				byteIdx += 1
			}
		}
	}

	switch v := value.(type) {
	case map[string]interface{}:
		t.nodeTypes[string(absKeypath)] = NodeTypeMap
		walkGoValue(value, func(nodeKeypath Keypath, nodeValue interface{}) error {
			absNodeKeypath := absKeypath.Push(nodeKeypath)
			newKeypaths = append(newKeypaths, absNodeKeypath)

			switch nodeValue.(type) {
			case map[string]interface{}:
				t.nodeTypes[string(absNodeKeypath)] = NodeTypeMap
			case []interface{}:
				t.nodeTypes[string(absNodeKeypath)] = NodeTypeSlice
			default:
				t.nodeTypes[string(absNodeKeypath)] = NodeTypeValue
				t.values[string(absNodeKeypath)] = nodeValue
			}
			return nil
		})

	case []interface{}:
		t.nodeTypes[string(absKeypath)] = NodeTypeSlice
		t.sliceLengths[string(absKeypath)] += len(v)
		walkGoValue(value, func(nodeKeypath Keypath, nodeValue interface{}) error {
			absNodeKeypath := absKeypath.Push(nodeKeypath)
			newKeypaths = append(newKeypaths, absNodeKeypath)

			switch nodeValue.(type) {
			case map[string]interface{}:
				t.nodeTypes[string(absNodeKeypath)] = NodeTypeMap
			case []interface{}:
				t.nodeTypes[string(absNodeKeypath)] = NodeTypeSlice
			default:
				t.nodeTypes[string(absNodeKeypath)] = NodeTypeValue
				t.values[string(absNodeKeypath)] = nodeValue
			}
			return nil
		})

	default:
		t.nodeTypes[string(absKeypath)] = NodeTypeValue
		_, exists := t.values[string(absKeypath)]
		t.values[string(absKeypath)] = value
		if !exists {
			newKeypaths = append(newKeypaths, absKeypath)
		}
	}
	t.diff.AddMany(newKeypaths)
	t.addKeypaths(newKeypaths)

	return nil
}

func (n *MemoryNode) Delete(keypath Keypath, rng *Range) error {
	if rng == nil {
		rng = n.rng
	} else if rng != nil && n.rng != nil {
		panic("unsupported")
	}
	if rng != nil && !rng.Valid() {
		return errors.WithStack(ErrInvalidRange)
	}

	n.checkCopied()

	absKeypath := n.keypath.Push(keypath)

	if rng == nil {
		delete(n.sliceLengths, string(absKeypath))
	} else {
		if !rng.Valid() {
			return ErrInvalidRange
		}

		switch n.nodeTypes[string(absKeypath)] {
		case NodeTypeSlice:
			n.sliceLengths[string(absKeypath)] -= int(rng.Size())
		case NodeTypeValue:
			if s, isString := n.values[string(absKeypath)].(string); isString {
				if !rng.ValidForLength(uint64(len(s))) {
					return ErrInvalidRange
				}
				startIdx, endIdx := rng.IndicesForLength(uint64(len(s)))
				n.values[string(absKeypath)] = s[:startIdx] + s[endIdx:]
				return nil

			} else {
				return ErrRangeOverNonSlice
			}

		default:
			return ErrRangeOverNonSlice
		}
	}

	startIdx := -1
	stopIdx := -1
	n.scanKeypathsWithPrefix(absKeypath, rng, func(keypath Keypath, i int) error {
		if startIdx == -1 {
			startIdx = i
		}
		stopIdx = i
		delete(n.values, string(keypath))
		delete(n.nodeTypes, string(keypath))
		delete(n.sliceLengths, string(keypath))
		return nil
	})
	var deletedKeypaths []Keypath
	if startIdx > -1 {
		deletedKeypaths = append(deletedKeypaths, n.keypaths[startIdx:stopIdx+1]...)
		n.keypaths = append(n.keypaths[:startIdx], n.keypaths[stopIdx+1:]...)
	}
	n.diff.RemoveMany(deletedKeypaths)
	return nil
}

func (n *MemoryNode) Diff() *Diff {
	return n.diff
}
func (n *MemoryNode) ResetDiff() {
	n.diff = NewDiff()
}

//func (t *Node) WalkDFS(fn func(node *Node) error) error {
//    node := &Node{store: t.store}
//    var prevKeypath Keypath
//    err := t.store.ScanKeypathsWithPrefix(t.Keypath, func(keypath Keypath, i int) error {
//        if prevKeypath != nil {
//            anc := prevKeypath.CommonAncestor(keypath)
//            var unresolvedChildPath Keypath
//            if len(anc) != 0 {
//                unresolvedChildPath, _ = prevKeypath[len(anc)+1:].Pop()
//            } else {
//                unresolvedChildPath, _ = prevKeypath[len(anc):].Pop()
//            }
//            numParts := unresolvedChildPath.NumParts()
//            remaining, _ := prevKeypath.Pop()
//            for j := numParts; j > 0; j-- {
//                node.Keypath = remaining
//                err := fn(node)
//                if err != nil {
//                    return err
//                }
//                remaining, _ = remaining.Pop()
//            }
//        }
//
//        node.Keypath = keypath
//        err := fn(node)
//        if err != nil {
//            return err
//        }
//
//        prevKeypath = keypath
//        return nil
//    })
//    if err != nil {
//        return err
//    }
//
//    remaining, _ := prevKeypath.Pop()
//    numParts := remaining.NumParts()
//    for j := numParts; j > 0; j-- {
//        node.Keypath = remaining
//        err := fn(node)
//        if err != nil {
//            return err
//        }
//        remaining, _ = remaining.Pop()
//    }
//
//    return nil
//}

type memoryDepthFirstIterator struct {
	i           int
	end         int
	prevLen     int
	backingNode *MemoryNode
	iterNode    *MemoryNode
}

func (t *MemoryNode) DepthFirstIterator(keypath Keypath, prefetchValues bool, prefetchSize int) Iterator {
	end, i := t.findPrefixRange(t.keypath.Push(keypath))

	return &memoryDepthFirstIterator{
		iterNode:    &MemoryNode{keypaths: t.keypaths, values: t.values, nodeTypes: t.nodeTypes},
		backingNode: t,
		i:           i,
		end:         end,
		prevLen:     len(t.keypaths),
	}
}

func (iter *memoryDepthFirstIterator) SeekTo(keypath Keypath) {
	newIdx := iter.end
	for i := len(iter.backingNode.keypaths) - 1; i >= 0; i-- {
		if iter.backingNode.keypaths[i].Equals(keypath) {
			newIdx = i
			break
		}
	}
	iter.i = newIdx
}

func (iter *memoryDepthFirstIterator) Next() Node {
	// Accounting to allow the consumer perform mutations without needing to manually seek the iterator.
	curLen := len(iter.backingNode.keypaths)
	if iter.prevLen != curLen {
		//iter.i += curLen - iter.prevLen
	}

	if iter.i == iter.end {
		return nil
	}

	iter.i--
	iter.iterNode.keypath = iter.backingNode.keypaths[iter.i]
	iter.prevLen = len(iter.backingNode.keypaths)
	return iter.iterNode
}

func (iter *memoryDepthFirstIterator) Close() {}

func (n *MemoryNode) MarshalJSON() ([]byte, error) {
	v, _, err := n.Value(nil, nil)
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func (s *MemoryNode) findPrefixStart(n int, prefix Keypath) int {
	if n >= len(s.keypaths) {
		return -1
	} else if len(prefix) == 0 {
		return 0
	} else if len(s.keypaths) == 0 {
		return -1
	}

	for i := n; i < len(s.keypaths); i++ {
		if s.keypaths[i].StartsWith(prefix) {
			return i
		}
	}
	return -1
}

func (s *MemoryNode) findPrefixEnd(n int, prefix Keypath) int {
	if n >= len(s.keypaths) {
		return -1
	}
	for i := n; i < len(s.keypaths); i++ {
		if !s.keypaths[i].StartsWith(prefix) {
			return i
		}
	}
	return -1
}

func (s *MemoryNode) findPrefixRange(prefix Keypath) (int, int) {
	if len(prefix) == 0 {
		return 0, len(s.keypaths)
	}

	start := -1
	for i := range s.keypaths {
		if s.keypaths[i].StartsWith(prefix) {
			if start == -1 {
				start = i
			}
		} else {
			if start != -1 {
				return start, i
			}
		}
	}
	if start == -1 {
		// The prefix was never found
		return -1, -1
	} else {
		// The prefix range extended to the end of s.keypaths
		return start, len(s.keypaths)
	}
}

func (s *MemoryNode) addKeypaths(keypaths []Keypath) {
	if len(keypaths) == 0 {
		return
	}

	// Sort the incoming keypaths first so that we don't have to sort the entire final list
	// @@TODO: sucks, write a quicksort without callbacks
	sort.Slice(keypaths, func(i, j int) bool { return bytes.Compare(keypaths[i], keypaths[j]) < 0 })

	start := s.findPrefixStart(0, keypaths[0])
	if start == -1 {
		start = 0
	}

	s.keypaths = append(s.keypaths, keypaths...)
	// @@TODO: sucks, write a quicksort without callbacks
	sort.Slice(s.keypaths[start:], func(i, j int) bool { return bytes.Compare(s.keypaths[i], s.keypaths[j]) < 0 })
}

func (s *MemoryNode) scanKeypathsWithPrefix(prefix Keypath, rng *Range, fn func(Keypath, int) error) error {
	if rng != nil {
		if s.nodeTypes[string(prefix)] != NodeTypeSlice {
			return ErrRangeOverNonSlice
		}

		startIdx, endIdx := rng.IndicesForLength(uint64(s.sliceLengths[string(prefix)]))
		startKeypathIdx := s.findPrefixStart(0, prefix.PushIndex(startIdx))
		if startKeypathIdx == -1 {
			return nil
		}
		endKeypathIdx := s.findPrefixEnd(startKeypathIdx+1, prefix.PushIndex(endIdx))
		if endKeypathIdx == -1 {
			panic("bad")
		}

		for i := startKeypathIdx; i < endKeypathIdx; i++ {
			err := fn(s.keypaths[i], i)
			if err != nil {
				return err
			}
			//i++
		}
	} else {
		var foundRange bool
		for i, keypath := range s.keypaths {
			if keypath.StartsWith(prefix) {
				foundRange = true
				err := fn(keypath, i)
				if err != nil {
					return err
				}
			} else if foundRange {
				return nil
			}
		}
	}
	return nil
}

func (s *MemoryNode) scanNodeTypesWithPrefix(prefix Keypath, fn func(Keypath, NodeType)) {
	for keypath, nodeType := range s.nodeTypes {
		if Keypath(keypath).StartsWith(prefix) {
			fn(Keypath(keypath), nodeType)
		}
	}
}

func (t *MemoryNode) DebugPrint() {
	fmt.Println("- root keypath:", t.keypath)
	fmt.Println("- copied:", t.copied)
	for _, kp := range t.keypaths {
		fmt.Println("  -", kp, t.nodeTypes[string(kp)], t.values[string(kp)])
	}
}
