package symbol_test

import (
	"bytes"
	"errors"
	"os"
	"path"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"redwood.dev/symbol"
)

var gTest = make(chan error)

func TestTable(t *testing.T) {

	go func() {
		doFullTableTest(800011)
		close(gTest)
	}()

	// wait for test to finish, fail the test on any errors
	err := <-gTest
	if err != nil {
		t.Fatal(err)
	}
}

func doFullTableTest(totalEntries int) {
	dir, err := os.MkdirTemp("", "junk*")
	if err != nil {
		gTest <- err
	}
	defer os.RemoveAll(dir)

	vals := make([][]byte, totalEntries)
	for i := range vals {
		vals[i] = []byte(strconv.Itoa(i))
	}
	IDs := make([]symbol.ID, totalEntries)

	// Test memory only mode
	fillTable(nil, vals, IDs)

	// Test WITH a database
	dbPathname := path.Join(dir, "test1")
	opts := badger.DefaultOptions(dbPathname)
	opts.Logger = nil

	// 1) tortuously fill and write a table
	{
		db, err := badger.Open(opts)
		if err != nil {
			gTest <- err
		}
		fillTable(db, vals, IDs)
		db.Close()
	}

	// 2) tortuously read and check the table
	{
		db, err := badger.Open(opts)
		if err != nil {
			gTest <- err
		}
		checkTable(db, vals, IDs)
		db.Close()
	}
}

const (
	hardwireStart     = symbol.MinIssuedID - hardwireTestCount
	hardwireTestCount = 101
)

func fillTable(db *badger.DB, vals [][]byte, IDs []symbol.ID) {
	totalEntries := len(vals)

	table, err := symbol.OpenTable(db, symbol.DefaultTableOpts)
	if err != nil {
		gTest <- err
	}
	defer table.Close()

	// Test reserved symbol ID space -- set symbol IDs less than symbol.MinIssuedID
	// Do multiple write passes to check overwrites don't cause issues.
	for k := 0; k < 3; k++ {
		for j := 0; j < hardwireTestCount; j++ {
			idx := hardwireStart + j
			symID := symbol.ID(idx)
			symID_got := table.SetSymbolID(vals[idx], symID)
			if symID_got != symID {
				gTest <- errors.New("SetSymbolID failed setup check")
			}
		}
	}

	hardwireCount := int32(0)
	hardwireCountPtr := &hardwireCount

	// Populate the table with multiple workers all setting values at once
	{
		running := &sync.WaitGroup{}
		numWorkers := 2
		for i := 0; i < numWorkers; i++ {
			running.Add(1)
			startAt := len(vals) * i / numWorkers
			go func() {
				for j := 0; j < totalEntries; j++ {
					idx := (startAt + j) % totalEntries
					symID := table.GetSymbolID(vals[idx], true)
					if symID < symbol.MinIssuedID {
						atomic.AddInt32(hardwireCountPtr, 1)
					}
					stored := table.LookupID(symID)
					if !bytes.Equal(stored, vals[idx]) {
						gTest <- errors.New("LookupID failed setup check")
					}
					symID_got := table.SetSymbolID(vals[idx], symID)
					if symID_got != symID {
						gTest <- errors.New("SetSymbolID failed setup check")
					}
				}
				running.Done()
			}()
		}

		running.Wait()

		if int(hardwireCount) != numWorkers*hardwireTestCount {
			gTest <- errors.New("hardwire test count failed")
		}

	}

	// Verify all the tokens are valid
	for i, k := range vals {
		IDs[i] = table.GetSymbolID(k, false)
		if IDs[i] == 0 {
			gTest <- errors.New("GetSymbolID failed final verification")
		}
		stored := table.LookupID(IDs[i])
		if !bytes.Equal(stored, vals[i]) {
			gTest <- errors.New("LookupID failed final verification")
		}
	}
}

func checkTable(db *badger.DB, vals [][]byte, IDs []symbol.ID) {
	totalEntries := len(vals)

	table, err := symbol.OpenTable(db, symbol.DefaultTableOpts)
	if err != nil {
		gTest <- err
	}
	defer table.Close()

	// Check that all the tokens are present
	{
		running := &sync.WaitGroup{}
		numWorkers := 5
		for i := 0; i < numWorkers; i++ {
			running.Add(1)
			startAt := len(vals) * i / numWorkers
			go func() {
				for j := 0; j < totalEntries; j++ {
					idx := (startAt + j) % totalEntries

					if (j % numWorkers) == 0 {
						symID := table.GetSymbolID(vals[idx], false)
						if symID != IDs[idx] {
							gTest <- errors.New("GetSymbolID failed readback check")
						}
					} else {
						stored := table.LookupID(IDs[idx])
						if !bytes.Equal(stored, vals[idx]) {
							gTest <- errors.New("LookupID failed readback check")
						}
					}
				}
				running.Done()
			}()
		}

		running.Wait()
	}
}
