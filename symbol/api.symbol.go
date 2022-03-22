package symbol

import (
	"github.com/dgraph-io/badger/v2"
)

// Creates a new symbol.Table and attaches it to the given db.
// If db == nil, the symbol.Table will operate in memory-only mode.
func OpenTable(db *badger.DB, opts TableOpts) (Table, error) {
	return openTable(db, opts)
}

// ID is a persistent integer value associated with an immutable string or buffer value.
// If an ID == 0, it denotes nil or unassigned.
type ID uint64

// IDSz is the byte size of a Table ID (big endian)
// This means an app issuing a new symbol ID every millisecond would need 35 years to hit the limit.
// Even in this case, the limiting factor is symbol storage (not an an ID issue limit of 2^40).
const IDSz = 5

// MinIssuedID specifies a minimum ID value for newly issued IDs.
//
// ID values less than this value are reserved for clients to represent hard-wired or "out of band" meaning.
// "Hard-wired" meaning that Table.SetSymbolID() can be used for IDs less than MinIssuedID with no risk
// of an ID being issued on top of it.
const MinIssuedID = 1000

// Table is a persistent storage of value-ID pairs, designed for high-performance lookup of an ID or buffer.
// This implementation is indended to handle extreme loads, leveraging:
//      - ID-value pairs are cached once read, offering subsequent O(1) access
//      - Internal value allocations are pooled. The default TableOpts.PoolSz of 16k means
//        thousands of buffers can be issued or read under only a single allocation.
//
// All calls are threadsafe.
type Table interface {

	// Returns the symbol ID previously associated with the given string/buffer value.
	// The given buffer is never retained.
	//
	// If not found and autoIssue == true, a new entry is created and the new ID returned.
	// Newly issued IDs are always > 0 and use the lower bytes of the returned ID (see type ID comments).
	//
	// If not found and autoIssue == false, 0 is returned.
	GetSymbolID(value []byte, autoIssue bool) ID

	// Associates the given buffer value to the given symbol ID, allowing multiple values to be mapped to a single ID.
	// If ID == 0, then this is the equivalent to GetSymbolID(value, true).
	SetSymbolID(value []byte, ID ID) ID

	// Returns the byte string previously associated with the given symbol ID.
	// If the ID is <= 0 or not found, nil is returned.
	// 
	// The buf returned conveniently retains scope until Close() is called (and is read-only). 
	LookupID(ID ID) []byte

	// Flushes any changes and internally closes
	// Any subsequent access to this interface is undefined.
	Close()
}

type TableOpts struct {
	WorkingSizeHint int   // anticipated number of entries in working set
	PoolSz          int32 // Value backing buffer allocation pool sz
	DbKeyPrefix     byte  // Only used in persistent db mode
}

// DefaultOpts is a suggested set of options.
var DefaultTableOpts = TableOpts{
	WorkingSizeHint: 600,
	PoolSz:          16 * 1024,
	DbKeyPrefix:     0xFA,
}
