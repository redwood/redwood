package symbol

import "unsafe"

// HashBuf is the hash function used by go map, it uses available hardware instructions.
// NOTE: The hash seed changes for every process. So, this cannot be used as a persistent hash.
func HashBuf(data []byte) uint64 {
	ss := (*stringStruct)(unsafe.Pointer(&data))
	return uint64(memhash(ss.str, 0, uintptr(ss.len)))
}

// HashStr is the hash function used by go map, it utilizes available hardware instructions.
// NOTE: The hash seed changes for every process. So, this cannot be used as a persistent hash.
func HashStr(str string) uint64 {
	ss := (*stringStruct)(unsafe.Pointer(&str))
	return uint64(memhash(ss.str, 0, uintptr(ss.len)))
}

// AP Hash Function -- deprecated in place of HashBuf.
// https://www.partow.net/programming/hashfunctions/#AvailableHashFunctions
func APHash64(buf []byte) uint64 {
	var hash uint64 = 0xaaaaaaaaaaaaaaaa
	for i, b := range buf {
		if (i & 1) == 0 {
			hash ^= ((hash << 7) ^ uint64(b) ^ (hash >> 3))
		} else {
			hash ^= (^((hash << 11) ^ uint64(b) ^ (hash >> 5)) + 1)
		}
	}
	return hash
}

type stringStruct struct {
	str unsafe.Pointer
	len int
}

//go:noescape
//go:linkname memhash runtime.memhash
func memhash(p unsafe.Pointer, h, s uintptr) uintptr
