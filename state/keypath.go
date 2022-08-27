package state

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"strconv"
	"strings"

	"redwood.dev/errors"
	"redwood.dev/types"
)

type Keypath []byte

var KeypathSeparator = Keypath("/")

const pathSepChar = byte('/')

func (k Keypath) Equals(other Keypath) bool {
	return bytes.Equal(k, other)
}

func (k Keypath) Copy() Keypath {
	k2 := make(Keypath, len(k))
	copy(k2, k)
	return k2
}

func (k Keypath) String() string {
	return string(k)
}

func (k Keypath) LengthAsParent() int {
	if len(k) != 0 {
		return len(k) + 1
	}
	return len(k)
}

func (k Keypath) ContainsSeparator() bool {
	return bytes.IndexByte(k, KeypathSeparator[0]) > -1
}

func (k Keypath) ContainsByte(b byte) bool {
	return bytes.IndexByte(k, b) > -1
}

func (k Keypath) IndexByte(b byte) int {
	return bytes.IndexByte(k, b)
}

func (k Keypath) ContainsPart(part Keypath) bool {
	idx := bytes.Index(k, part)
	if idx == -1 {
		return false
	}
	if idx == 0 {
		return len(k) == len(part) || k[len(part)] == KeypathSeparator[0]
	} else if idx == len(k)-len(part) {
		return k[idx-1] == KeypathSeparator[0]
	} else {
		return k[idx-1] == KeypathSeparator[0] && k[idx+len(part)] == KeypathSeparator[0]
	}
}

func (k Keypath) RelativeTo(root Keypath) Keypath {
	x := k[len(root):]
	if len(x) > 0 && x[0] == KeypathSeparator[0] {
		return x[1:]
	}
	return x
}

func (k Keypath) FirstNParts(n int) Keypath {
	if n == 0 {
		return nil
	}

	current := k
	var endIdx int
	for i := 0; i < n; i++ {
		idx := bytes.IndexByte(current, KeypathSeparator[0])
		current = current[idx+1:]
		endIdx += idx

		if i != 0 {
			endIdx++
		}
	}

	return k[:endIdx]
}

// LastNParts returns the N right-most Keypath components (or nil if there less than the requested number of components).
// Any trailing path sep chars are effectively ignored.
//
// "1/22/333".LastNParts(1)  =>  "333"
//
// "1/22/333".LastNParts(2)  =>  "22/333"
//
// "1/22/333/".LastNParts(2)  =>  "22/333"
//
// "1/22/333/".LastNParts(4)  =>  nil
func (k Keypath) LastNParts(n int) Keypath {
	if n <= 0 {
		return nil
	}

	klen := len(k)
	for ; klen > 0; klen-- {
		if k[klen-1] != '/' {
			break
		}
	}

	for idx := klen - 1; idx >= 0; idx-- {
		if k[idx] == pathSepChar {
			n--
			if n == 0 {
				return k[idx+1 : klen]
			}
		}
	}

	if n == 1 {
		return k[:klen]
	}

	return nil
}

func (k Keypath) StartsWith(prefixParts Keypath) bool {
	if len(prefixParts) == 0 {
		return true
	} else if len(k) == len(prefixParts) {
		return bytes.Equal(k, prefixParts)
	} else if len(prefixParts) > len(k) {
		return false
	}
	return bytes.HasPrefix(k, prefixParts) && k[len(prefixParts)] == KeypathSeparator[0]
}

func (k Keypath) Unshift(part Keypath) Keypath {
	if len(k) == 0 {
		return part
	}
	k2 := make(Keypath, len(k)+len(part)+1)
	copy(k2, part)
	k2[len(part)] = KeypathSeparator[0]
	copy(k2[len(part)+1:], k)
	return k2
}

func (k Keypath) Shift() (top Keypath, rest Keypath) {
	kpIdx := bytes.IndexByte(k, KeypathSeparator[0])
	if kpIdx == -1 {
		return k, nil
	}
	return k[:kpIdx], k[kpIdx+1:]
}

func (k Keypath) Push(part Keypath) Keypath {
	if len(k) == 0 {
		return part
	} else if len(part) == 0 {
		return k
	}
	k2 := make(Keypath, len(k)+len(part)+1)
	copy(k2, k)
	k2[len(k)] = KeypathSeparator[0]
	copy(k2[len(k)+1:], part)
	return k2
}

func (k Keypath) Pushs(part string) Keypath {
	return k.Push(Keypath(part))
}

func (k Keypath) Pushb(part []byte) Keypath {
	return k.Push(Keypath(part))
}

func (k Keypath) PushIndex(idx uint64) Keypath {
	return k.Push(EncodeSliceIndex(idx))
}

func (k Keypath) Pop() (rest Keypath, top Keypath) {
	kpIdx := bytes.LastIndexByte(k, KeypathSeparator[0])
	if kpIdx == -1 {
		return nil, k
	}
	return k[:kpIdx], k[kpIdx+1:]
}

func (k Keypath) NumParts() int {
	if len(k) == 0 {
		return 0
	}
	return bytes.Count(k, KeypathSeparator) + 1
}

func (k Keypath) Part(partIdx int) Keypath {
	if partIdx < 0 {
		byteIdx := len(k)
		prevByteIdx := len(k)
		for i := -1; i >= partIdx; i-- {
			newByteIdx := bytes.LastIndexByte(k[:byteIdx], KeypathSeparator[0])
			prevByteIdx = byteIdx
			byteIdx = newByteIdx
			if newByteIdx == -1 {
				if i == partIdx {
					return k[byteIdx+1 : prevByteIdx]
				}
				return nil
			}
		}
		return k[byteIdx+1 : prevByteIdx]

	} else {
		byteIdx := -1
		prevByteIdx := 0
		for i := 0; i <= partIdx; i++ {
			prevByteIdx = byteIdx + 1
			newByteIdx := bytes.IndexByte(k[byteIdx+1:], KeypathSeparator[0])
			if newByteIdx == -1 {
				if i == partIdx {
					return k[prevByteIdx:]
				}
				return nil
			}
			newByteIdx += prevByteIdx
			byteIdx = newByteIdx
		}
		return k[prevByteIdx:byteIdx]
	}
	return nil
}

func (k Keypath) Parts() []Keypath {
	if len(k) == 0 {
		return nil
	}

	n := k.NumParts()
	a := make([]Keypath, n)
	if n == 1 {
		a[0] = k
		return a
	}

	n--
	i := 0
	for {
		m := bytes.IndexByte(k, KeypathSeparator[0])
		if m < 0 {
			break
		}
		a[i] = k[:m:m]
		k = k[m+1:]
		i++
	}
	a[i] = k
	return a[:i+1]
}

func (k Keypath) PartStrings() []string {
	parts := k.Parts()
	if len(parts) == 0 {
		return nil
	}
	partStrings := make([]string, len(parts))
	for i, part := range parts {
		partStrings[i] = string(part)
	}
	return partStrings
}

func (k Keypath) CommonAncestor(other Keypath) Keypath {
	var lastSeparatorIdx int
	long := k
	short := other
	if len(long) < len(short) {
		long, short = short, long
	}
	for i := range short {
		if short[i] != long[i] {
			return short[:lastSeparatorIdx]
		} else if short[i] == KeypathSeparator[0] {
			lastSeparatorIdx = i
		}
	}
	return short
}

func (k Keypath) Normalized() Keypath {
	if len(k) == 0 {
		return nil
	}
	if k[0] == KeypathSeparator[0] {
		k = k[1:]
	}
	if len(k) == 0 {
		return nil
	}
	if k[len(k)-1] == KeypathSeparator[0] {
		k = k[:len(k)-1]
	}
	return k
}

func (k Keypath) Marshal() ([]byte, error) {
	k2 := make(Keypath, len(k))
	copy(k2, k)
	return k2, nil
}

func (k *Keypath) MarshalTo(data []byte) (n int, err error) {
	copy(data, *k)
	return len(data), nil
}

func (k *Keypath) Unmarshal(data []byte) error {
	*k = make(Keypath, len(data))
	copy(*k, Keypath(data).Normalized())
	return nil
}

func (k *Keypath) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	*k = Keypath(s).Normalized()
	return nil
}

func (k Keypath) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(k.Normalized()))
}

func (k *Keypath) Size() int { return len(*k) }

func (k Keypath) Compare(other Keypath) int { return bytes.Compare(k[:], other[:]) }
func (k Keypath) Equal(other Keypath) bool  { return bytes.Equal(k[:], other[:]) }

func JoinKeypaths(s []Keypath) Keypath {
	if len(s) == 0 {
		return nil
	}
	if len(s) == 1 {
		// Just return a copy.
		return append([]byte(nil), s[0]...)
	}
	n := len(KeypathSeparator) * (len(s) - 1)
	for _, v := range s {
		n += len(v)
	}

	b := make(Keypath, n)
	bp := copy(b, s[0])
	for _, v := range s[1:] {
		bp += copy(b[bp:], KeypathSeparator)
		bp += copy(b[bp:], v)
	}
	return b
}

type gogoprotobufTest interface {
	Float32() float32
	Float64() float64
	Int63() int64
	Int31() int32
	Uint32() uint32
	Intn(n int) int
}

func NewPopulatedKeypath(_ gogoprotobufTest) *Keypath {
	parts := make([]string, rand.Intn(5))
	for i := range parts {
		parts[i] = types.RandomString(rand.Intn(10))
	}
	k := Keypath([]byte(strings.Join(parts, string(KeypathSeparator))))
	return &k
}

var ErrBadKeypath = errors.New("bad keypath")

func ParseKeypathAndRange(s []byte, keypathSeparator byte) (Keypath, *Range, error) {
	var keypath Keypath
	var rng *Range

	s = bytes.TrimSpace(s)

	for i := 0; i < len(s); {
		switch s[i] {
		case keypathSeparator:
			key, err := parseKeypathPart(s[i:], keypathSeparator)
			if err != nil {
				return nil, nil, errors.WithStack(err)
			}
			keypath = keypath.Push(key)
			i += len(key) + 1

		case '[':
			switch s[i+1] {
			case '"', '\'':
				key, err := parseBracketKey(s[i:])
				if err != nil {
					return nil, nil, errors.WithStack(err)
				}
				keypath = keypath.Push(key)
				i += len(key) + 4

			case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				theRange, idx, length, err := parseRangeOrIndex(s[i:])
				if err != nil {
					return nil, nil, errors.WithStack(err)
				}
				if theRange != nil {
					rng = theRange
				} else {
					keypath = keypath.PushIndex(idx)
				}
				i += length

			default:
				return nil, nil, errors.WithStack(ErrBadKeypath)
			}

		default:
			return nil, nil, errors.WithStack(ErrBadKeypath)
		}
	}
	return keypath.Normalized(), rng, nil
}

func parseKeypathPart(s []byte, keypathSeparator byte) ([]byte, error) {
	buf := []byte{}
	// start at index 1, skip first dot
	for i := 1; i < len(s); i++ {
		if s[i] == keypathSeparator || s[i] == '[' || s[i] == ' ' {
			if len(buf) == 0 {
				return nil, nil
			}
			return buf, nil
		} else {
			buf = append(buf, s[i])
		}
	}
	return buf, nil
}

func parseBracketKey(s []byte) ([]byte, error) {
	if len(s) < 5 {
		return nil, errors.WithStack(ErrBadKeypath)
	} else if s[0] != '[' && s[1] != '"' {
		return nil, errors.WithStack(ErrBadKeypath)
	}

	buf := []byte{}
	// start at index 2, skip ["
	for i := 2; i < len(s); i++ {
		if s[i] == '"' {
			return buf, nil
		} else {
			buf = append(buf, s[i])
		}
	}
	return nil, errors.WithStack(ErrBadKeypath)
}

func parseRangeOrIndex(s []byte) (*Range, uint64, int, error) {
	var (
		isRange = false
		rng     = &Range{}
		buf     = make([]byte, 0, 8) // Approximation/heuristic
	)
	// Start at index 1, skip [
	for i := 1; i < len(s); i++ {
		if s[i] == ']' {
			if len(buf) == 0 {
				return nil, 0, 0, errors.WithStack(ErrBadKeypath)
			}
			end, err := strconv.ParseInt(string(buf), 10, 64)
			if err != nil {
				return nil, 0, 0, errors.WithStack(ErrBadKeypath)
			}

			if !isRange {
				if end < 0 {
					// Can't have negative indices (yet... @@TODO)
					return nil, 0, 0, errors.WithStack(ErrBadKeypath)
				}
				return nil, uint64(end), i + 1, nil
			}

			if end == 0 && buf[0] == '-' {
				rng.Reverse = true
			} else if end < 0 {
				rng.End = uint64(-end)
				rng.Reverse = true
			} else {
				rng.End = uint64(end)
			}
			return rng, 0, i + 1, nil

		} else if s[i] == ':' {
			if isRange {
				// Disallow [x:y:z]
				return nil, 0, 0, errors.WithStack(ErrBadKeypath)
			}

			start, err := strconv.ParseInt(string(buf), 10, 64)
			if err != nil {
				return nil, 0, 0, errors.WithStack(ErrBadKeypath)
			}
			if start == 0 && buf[0] == '-' {
				rng.Reverse = true
			} else if start < 0 {
				rng.Start = uint64(-start)
				rng.Reverse = true
			} else {
				rng.Start = uint64(start)
			}
			isRange = true
			buf = make([]byte, 0, 8)
		} else {
			buf = append(buf, s[i])
		}
	}
	return nil, 0, 0, errors.WithStack(ErrBadKeypath)
}
