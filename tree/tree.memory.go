package tree

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/types"
)

var log = ctx.NewLogger("memory node")

type MemoryNode struct {
	keypath Keypath
	rng     *Range

	keypaths       []Keypath
	values         map[string]interface{}
	nodeTypes      map[string]NodeType
	contentLengths map[string]uint64
	copied         bool
	diff           *Diff
}

func NewMemoryNode() Node {
	return &MemoryNode{
		keypath:        nil,
		rng:            nil,
		keypaths:       nil,
		values:         make(map[string]interface{}),
		nodeTypes:      make(map[string]NodeType),
		contentLengths: make(map[string]uint64),
		copied:         false,
		diff:           NewDiff(),
	}
}

func (n *MemoryNode) Close() {}

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

	cpy := &MemoryNode{
		keypath:        t.keypath.Push(keypath),
		rng:            rng,
		keypaths:       t.keypaths,
		values:         t.values,
		nodeTypes:      t.nodeTypes,
		contentLengths: t.contentLengths,
		diff:           t.diff,
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
	contentLengths := make(map[string]uint64)

	copy(keypaths, t.keypaths[start:end])

	for _, kp := range keypaths {
		values[string(kp)] = t.values[string(kp)]
		nodeTypes[string(kp)] = t.nodeTypes[string(kp)]
		if nodeTypes[string(kp)] == NodeTypeSlice || nodeTypes[string(kp)] == NodeTypeMap {
			contentLengths[string(kp)] = t.contentLengths[string(kp)]
		}
	}

	t.keypaths = keypaths
	t.values = values
	t.nodeTypes = nodeTypes
	t.contentLengths = contentLengths
	t.diff = t.diff.Copy()

	t.copied = false
}

// Subkeys returns only the keys that are direct descendants of the
// given node.
func (n *MemoryNode) Subkeys() []Keypath {
	var keypaths []Keypath
	keypathsMap := make(map[string]struct{})
	_ = n.scanKeypathsWithPrefix(n.keypath, nil, func(kp Keypath, _ int) error {
		subkey := kp.RelativeTo(n.keypath).Part(0)
		_, exists := keypathsMap[string(subkey)]
		if !exists && len(subkey) > 0 {
			keypaths = append(keypaths, subkey)
			keypathsMap[string(subkey)] = struct{}{}
		}
		return nil
	})
	return keypaths
}

// NodeAt returns the tree.Node corresponding to the given keypath in
// the state tree.  If the keypath doesn't exist, a MemoryNode is still
// returned, but calling .Value on it will return a result with
// NodeTypeInvalid.
func (n *MemoryNode) NodeAt(relKeypath Keypath, rng *Range) Node {
	absKeypath := n.keypath.Push(relKeypath)

	if node, relKeypath := n.ParentNodeFor(relKeypath); n != node {
		return node.NodeAt(relKeypath, rng)
	}

	if n.nodeTypes[string(absKeypath)] == NodeTypeNode {
		// This shouldn't happen because we call .ParentNodeFor
		return n.values[string(absKeypath)].(Node)
	}

	return &MemoryNode{
		keypath:        absKeypath,
		rng:            rng,
		keypaths:       n.keypaths,
		values:         n.values,
		nodeTypes:      n.nodeTypes,
		contentLengths: n.contentLengths,
		diff:           n.diff,
	}
}

// ParentNodeFor in most cases returns the method's receiver.  But in
// the case where another tree.Node has been .Set into the receiver's
// state tree at some point in the provided keypath, it will return the
// deepest tree.Node corresponding to that keypath.  The returned keypath
// is provided keypath, relative to the deepest tree.Node.
func (n *MemoryNode) ParentNodeFor(relKeypath Keypath) (Node, Keypath) {
	if len(relKeypath) == 0 {
		return n, relKeypath
	}

	parts := append([]Keypath{nil}, relKeypath.Parts()...)
	absKeypath := n.keypath.Push(relKeypath)
	currentKeypath := n.keypath.Copy()
	for _, part := range parts {
		currentKeypath = currentKeypath.Push(part)

		switch n.nodeTypes[string(currentKeypath)] {
		case NodeTypeNode:
			innerNode := n.values[string(currentKeypath)].(Node)
			relKeypath := absKeypath.RelativeTo(currentKeypath)
			return innerNode.ParentNodeFor(relKeypath)

		case NodeTypeValue, NodeTypeInvalid:
			return n, relKeypath
		}
	}
	return n, relKeypath
}

func (n *MemoryNode) nodeType(keypath Keypath) NodeType {
	return n.nodeTypes[string(n.keypath.Push(keypath))]
}

// NodeInfo returns metadata about the node: its NodeType, its ValueType
// (if applicable), and the Content-Length of its value.
func (n *MemoryNode) NodeInfo(keypath Keypath) (NodeType, ValueType, uint64, error) {
	if node, relKeypath := n.ParentNodeFor(keypath); n != node {
		return node.NodeInfo(relKeypath)
	}

	absKeypath := n.keypath.Push(keypath)

	switch n.nodeTypes[string(absKeypath)] {
	case NodeTypeInvalid:
		return 0, 0, 0, errors.WithStack(types.Err404)

	case NodeTypeMap:
		return NodeTypeMap, 0, uint64(n.contentLengths[string(absKeypath)]), nil

	case NodeTypeSlice:
		return NodeTypeSlice, 0, uint64(n.contentLengths[string(absKeypath)]), nil

	case NodeTypeValue:
		val, exists := n.values[string(absKeypath)]
		if !exists {
			return 0, 0, 0, errors.WithStack(types.Err404)
		}
		switch v := val.(type) {
		case string:
			return NodeTypeValue, ValueTypeString, uint64(len(v)), nil
		case float64:
			return NodeTypeValue, ValueTypeFloat, 0, nil
		case uint64:
			return NodeTypeValue, ValueTypeUint, 0, nil
		case int64:
			return NodeTypeValue, ValueTypeInt, 0, nil
		case bool:
			return NodeTypeValue, ValueTypeBool, 0, nil
		case nil:
			return NodeTypeValue, ValueTypeNil, 0, nil
		default:
			return NodeTypeValue, ValueTypeInvalid, 0, nil
		}
	case NodeTypeNode:
		node, is := n.values[string(absKeypath)].(Node)
		if !is {
			panic("corrupted MemoryNode")
		}
		return node.NodeInfo(nil)
	}
	panic("unreachable")
}

// Exists returns a boolean representing whether the given keypath has
// been set in the subtree for this node.  For a MemoryNode, it never
// returns an error.
func (n *MemoryNode) Exists(keypath Keypath) (bool, error) {
	if node, relKeypath := n.ParentNodeFor(keypath); n != node {
		return node.Exists(relKeypath)
	}

	absKeypath := n.keypath.Push(keypath)

	_, exists := n.nodeTypes[string(absKeypath)]
	return exists, nil
}

func (n *MemoryNode) UintValue(keypath Keypath) (uint64, bool, error) {
	if node, relKeypath := n.ParentNodeFor(keypath); n != node {
		return node.UintValue(relKeypath)
	}

	absKeypath := n.keypath.Push(keypath)

	v, exists := n.values[string(absKeypath)]
	if !exists {
		return 0, false, nil
	}
	if asUint, isUint := v.(uint64); isUint {
		return asUint, true, nil
	}
	return 0, false, nil
}

func (n *MemoryNode) IntValue(keypath Keypath) (int64, bool, error) {
	if node, relKeypath := n.ParentNodeFor(keypath); n != node {
		return node.IntValue(relKeypath)
	}

	absKeypath := n.keypath.Push(keypath)

	v, exists := n.values[string(absKeypath)]
	if !exists {
		return 0, false, nil
	}
	if asInt, isInt := v.(int64); isInt {
		return asInt, true, nil
	}
	return 0, false, nil
}

func (n *MemoryNode) FloatValue(keypath Keypath) (float64, bool, error) {
	if node, relKeypath := n.ParentNodeFor(keypath); n != node {
		return node.FloatValue(relKeypath)
	}

	absKeypath := n.keypath.Push(keypath)

	v, exists := n.values[string(absKeypath)]
	if !exists {
		return 0, false, nil
	}
	if asFloat, isFloat := v.(float64); isFloat {
		return asFloat, true, nil
	}
	return 0, false, nil
}

func (n *MemoryNode) StringValue(keypath Keypath) (string, bool, error) {
	if node, relKeypath := n.ParentNodeFor(keypath); n != node {
		return node.StringValue(relKeypath)
	}

	absKeypath := n.keypath.Push(keypath)

	v, exists := n.values[string(absKeypath)]
	if !exists {
		return "", false, nil
	}
	if asString, isString := v.(string); isString {
		return asString, true, nil
	}
	return "", false, nil
}

// Value returns the native Go value at the given keypath and range.
func (n *MemoryNode) Value(keypath Keypath, rng *Range) (interface{}, bool, error) {
	if node, relKeypath := n.ParentNodeFor(keypath); n != node {
		return node.Value(relKeypath, rng)
	}

	absKeypath := n.keypath.Push(keypath)

	if rng == nil {
		rng = n.rng
	} else if rng != nil && n.rng != nil {
		panic("unsupported")
	}
	if rng != nil && !rng.Valid() {
		return nil, false, errors.WithStack(ErrInvalidRange)
	}

	switch n.nodeTypes[string(absKeypath)] {
	case NodeTypeInvalid:
		return nil, false, nil

	case NodeTypeNode:
		// This should never happen because we call .ParentNodeFor above
		return n.values[string(absKeypath)].(Node).Value(keypath, rng)

	case NodeTypeValue:
		val, exists := n.values[string(absKeypath)]
		if !exists {
			return nil, false, nil
		} else if rng != nil {
			// @@TODO: support strings/bytes
			return nil, false, ErrRangeOverNonSlice
		}
		return val, exists, nil

	case NodeTypeMap:
		if rng != nil {
			return nil, false, ErrRangeOverNonSlice
		}

		m := make(map[string]interface{}, n.contentLengths[string(absKeypath)])

		n.scanKeypathsWithPrefix(absKeypath, nil, func(kp Keypath, _ int) error {
			relKp := kp.RelativeTo(absKeypath)

			if len(relKp) != 0 {
				switch n.nodeTypes[string(kp)] {
				case NodeTypeSlice:
					setValueAtKeypath(m, relKp, make([]interface{}, n.contentLengths[string(kp)]), false)
				case NodeTypeMap:
					setValueAtKeypath(m, relKp, make(map[string]interface{}, n.contentLengths[string(kp)]), false)
				case NodeTypeNode:
					node := n.values[string(kp)].(Node)
					val, exists, err := node.Value(nil, nil)
					if err != nil {
						return err
					} else if !exists {
						panic("wat")
					}
					setValueAtKeypath(m, relKp, val, false)
				case NodeTypeValue:
					setValueAtKeypath(m, relKp, n.values[string(kp)], false)
				}
			}
			return nil
		})
		return m, true, nil

	case NodeTypeSlice:
		s := make([]interface{}, n.contentLengths[string(absKeypath)])

		n.scanKeypathsWithPrefix(absKeypath, rng, func(kp Keypath, _ int) error {
			relKp := kp.RelativeTo(absKeypath)

			if len(relKp) != 0 {
				switch n.nodeTypes[string(kp)] {
				case NodeTypeSlice:
					setValueAtKeypath(s, relKp, make([]interface{}, n.contentLengths[string(kp)]), false)
				case NodeTypeMap:
					setValueAtKeypath(s, relKp, make(map[string]interface{}, n.contentLengths[string(kp)]), false)
				case NodeTypeNode:
					node := n.values[string(kp)].(Node)
					val, exists, err := node.Value(nil, nil)
					if err != nil {
						return err
					} else if !exists {
						panic("wat")
					}
					setValueAtKeypath(s, relKp, val, false)
				case NodeTypeValue:
					setValueAtKeypath(s, relKp, n.values[string(kp)], false)
				}
			}
			return nil
		})
		return s, true, nil

	default:
		panic("tree.Value(): bad NodeType")
	}
}

func (n *MemoryNode) Length() (uint64, error) {
	switch n.nodeTypes[string(n.keypath)] {
	case NodeTypeMap:
		return n.contentLengths[string(n.keypath)], nil
	case NodeTypeSlice:
		return n.contentLengths[string(n.keypath)], nil
	case NodeTypeValue:
		switch v := n.values[string(n.keypath)].(type) {
		case string:
			return uint64(len(v)), nil
		default:
			return 0, nil
		}
	default:
		return 0, nil
	}
}

func (t *MemoryNode) Set(keypath Keypath, rng *Range, value interface{}) error {
	if node, relKeypath := t.ParentNodeFor(keypath); t != node {
		return node.Set(relKeypath, rng, value)
	}

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
		parts := append([]Keypath{nil}, absKeypath.Parts()...)
		var byteIdx int
		for i, key := range parts[:len(parts)-1] {
			byteIdx += len(key)

			var partialKeypath Keypath
			// We always want 0-length keypaths to be nil
			if byteIdx != 0 {
				partialKeypath = absKeypath[:byteIdx]
			}

			nt := t.nodeTypes[string(partialKeypath)]
			if nt == NodeTypeInvalid {
				t.nodeTypes[string(partialKeypath)] = NodeTypeMap
				newKeypaths = append(newKeypaths, partialKeypath)
			}

			if i != 0 {
				// Only account for the keypath separator after the root (nil) keypath
				byteIdx += 1
			}
		}
	}

	walkGoValue(value, func(nodeKeypath Keypath, nodeValue interface{}) error {
		absNodeKeypath := absKeypath.Push(nodeKeypath)
		newKeypaths = append(newKeypaths, absNodeKeypath)

		switch nv := nodeValue.(type) {
		case map[string]interface{}:
			t.nodeTypes[string(absNodeKeypath)] = NodeTypeMap
			t.contentLengths[string(absNodeKeypath)] = uint64(len(nv))
		case []interface{}:
			t.nodeTypes[string(absNodeKeypath)] = NodeTypeSlice
			t.contentLengths[string(absNodeKeypath)] = uint64(len(nv))
		case Node:
			t.nodeTypes[string(absNodeKeypath)] = NodeTypeNode
			t.values[string(absNodeKeypath)] = nodeValue
		default:
			t.nodeTypes[string(absNodeKeypath)] = NodeTypeValue
			t.values[string(absNodeKeypath)] = nodeValue
		}
		return nil
	})

	t.diff.AddMany(newKeypaths)
	t.addKeypaths(newKeypaths)

	return nil
}

func (n *MemoryNode) Delete(keypath Keypath, rng *Range) error {
	if node, relKeypath := n.ParentNodeFor(keypath); n != node {
		return node.Delete(relKeypath, rng)
	}

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
		delete(n.contentLengths, string(absKeypath))
	} else {
		if !rng.Valid() {
			return ErrInvalidRange
		}

		switch n.nodeTypes[string(absKeypath)] {
		case NodeTypeMap:
			n.contentLengths[string(absKeypath)] -= rng.Size()
		case NodeTypeSlice:
			n.contentLengths[string(absKeypath)] -= rng.Size()
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
		delete(n.contentLengths, string(keypath))
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

type memoryIterator struct {
	i           int
	start, end  int
	done        bool
	rootKeypath Keypath
	iterNode    *MemoryNode
}

func (n *MemoryNode) Iterator(keypath Keypath, prefetchValues bool, prefetchSize int) Iterator {
	if node, relKeypath := n.ParentNodeFor(keypath); n != node {
		return node.Iterator(relKeypath, prefetchValues, prefetchSize)
	}

	start, end := n.findPrefixRange(n.keypath.Push(keypath))

	return &memoryIterator{
		iterNode:    &MemoryNode{keypaths: n.keypaths, values: n.values, nodeTypes: n.nodeTypes, contentLengths: n.contentLengths},
		rootKeypath: n.keypath.Push(keypath),
		start:       start,
		end:         end,
	}
}

func (iter *memoryIterator) noItems() bool {
	return iter.start == -1 && iter.end == -1
}

func (iter *memoryIterator) Rewind() {
	if iter.noItems() {
		iter.done = true
		return
	}
	iter.i = iter.start
	iter.done = false
	iter.iterNode.keypath = iter.iterNode.keypaths[iter.i]
}

func (iter *memoryIterator) Valid() bool {
	if iter.noItems() {
		iter.done = true
		return false
	}
	return !iter.done
}

func (iter *memoryIterator) SeekTo(keypath Keypath) {
	if iter.noItems() {
		iter.done = true
		return
	}
	newIdx := 0
	found := false
	for i := 0; i < len(iter.iterNode.keypaths); i++ {
		if iter.iterNode.keypaths[i].Equals(keypath) {
			newIdx = i
			found = true
			break
		}
	}
	if found {
		iter.i = newIdx
		iter.done = false
		iter.iterNode.keypath = iter.iterNode.keypaths[iter.i]
	} else {
		iter.done = true
	}
}

func (iter *memoryIterator) Node() Node {
	if iter.done || iter.noItems() {
		return nil
	}
	return iter.iterNode
}

func (iter *memoryIterator) Next() {
	if iter.done || iter.noItems() {
		return
	} else if iter.i == iter.end-1 {
		iter.done = true
		return
	}
	iter.i++
	iter.iterNode.keypath = iter.iterNode.keypaths[iter.i]
}

func (iter *memoryIterator) Close() {
	iter.done = true
}

type memoryChildIterator struct {
	*memoryIterator
}

func (n *MemoryNode) ChildIterator(keypath Keypath, prefetchValues bool, prefetchSize int) Iterator {
	if node, relKeypath := n.ParentNodeFor(keypath); n != node {
		return node.ChildIterator(relKeypath, prefetchValues, prefetchSize)
	}
	return &memoryChildIterator{n.Iterator(keypath, prefetchValues, prefetchSize).(*memoryIterator)}
}

func (iter *memoryChildIterator) Node() Node {
	return iter.memoryIterator.Node()
}

func (iter *memoryChildIterator) Rewind() {
	iter.memoryIterator.Rewind()
	iter.Next()
}

func (iter *memoryChildIterator) Next() {
	iter.memoryIterator.Next()
	for ; iter.memoryIterator.Valid(); iter.memoryIterator.Next() {
		node := iter.memoryIterator.Node()
		if node.Keypath().NumParts() == iter.memoryIterator.rootKeypath.NumParts()+1 {
			return
		}
	}
}

type memoryDepthFirstIterator struct {
	i           int
	start, end  int
	done        bool
	backingNode *MemoryNode
	iterNode    *MemoryNode
}

func (n *MemoryNode) DepthFirstIterator(keypath Keypath, prefetchValues bool, prefetchSize int) Iterator {
	if node, relKeypath := n.ParentNodeFor(keypath); n != node {
		return node.DepthFirstIterator(relKeypath, prefetchValues, prefetchSize)
	}
	end, start := n.findPrefixRange(n.keypath.Push(keypath))

	return &memoryDepthFirstIterator{
		iterNode:    &MemoryNode{keypaths: n.keypaths, values: n.values, nodeTypes: n.nodeTypes, contentLengths: n.contentLengths},
		backingNode: n,
		start:       start,
		end:         end,
	}
}

func (iter *memoryDepthFirstIterator) noItems() bool {
	return iter.start == -1 && iter.end == -1
}

func (iter *memoryDepthFirstIterator) SeekTo(keypath Keypath) {
	if iter.noItems() {
		iter.done = true
		return
	}

	newIdx := iter.end
	found := false
	for i := len(iter.backingNode.keypaths) - 1; i >= 0; i-- {
		if iter.backingNode.keypaths[i].Equals(keypath) {
			newIdx = i
			found = true
			break
		}
	}
	if found {
		iter.i = newIdx
		iter.done = false
		iter.iterNode.keypath = iter.backingNode.keypaths[iter.i]
	} else {
		iter.done = true
	}
}

func (iter *memoryDepthFirstIterator) Valid() bool {
	if iter.noItems() {
		iter.done = true
		return false
	}
	return !iter.done
}

func (iter *memoryDepthFirstIterator) Rewind() {
	if iter.noItems() {
		iter.done = true
		return
	}
	iter.i = iter.start - 1
	iter.done = false
	iter.iterNode.keypath = iter.backingNode.keypaths[iter.i]
}

func (iter *memoryDepthFirstIterator) Node() Node {
	if iter.done || iter.noItems() {
		return nil
	}
	return iter.iterNode
}

func (iter *memoryDepthFirstIterator) Next() {
	if iter.done || iter.noItems() {
		return
	} else if iter.i == iter.end {
		iter.done = true
		return
	}

	iter.i--
	iter.iterNode.keypath = iter.backingNode.keypaths[iter.i]
}

func (iter *memoryDepthFirstIterator) Close() {
	iter.done = true
}

func (n *MemoryNode) UnmarshalJSON(bs []byte) error {
	if n == nil {
		newNode := NewMemoryNode().(*MemoryNode)
		*n = *newNode
	}

	var val interface{}
	err := json.Unmarshal(bs, &val)
	if err != nil {
		return err
	}
	return n.Set(nil, nil, val)
}

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

		startIdx, endIdx := rng.IndicesForLength(uint64(s.contentLengths[string(prefix)]))
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

func (t *MemoryNode) DebugPrint() {
	fmt.Println("MemoryNode ----------------------------------------")
	fmt.Println("- root keypath: ", t.keypath)
	fmt.Println("- copied: ", t.copied)
	for _, kp := range t.keypaths {
		if t.nodeTypes[string(kp)] == NodeTypeSlice || t.nodeTypes[string(kp)] == NodeTypeMap {
			fmt.Printf("  - %v %v %v %v\n", kp, t.nodeTypes[string(kp)], t.values[string(kp)], t.contentLengths[string(kp)])
		} else {
			fmt.Printf("  - %v %v %v (%T)\n", kp, t.nodeTypes[string(kp)], t.values[string(kp)], t.values[string(kp)])
		}
	}
	fmt.Println("---------------------------------------------------")
}
