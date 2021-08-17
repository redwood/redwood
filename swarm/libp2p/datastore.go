package libp2p

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
	dstore "github.com/ipfs/go-datastore"
	recpb "github.com/libp2p/go-libp2p-record/pb"

	"redwood.dev/log"
)

type notifyingDatastore struct {
	dstore.Batching
}

func (ds *notifyingDatastore) Put(k dstore.Key, v []byte) error {
	err := ds.Batching.Put(k, v)
	if err != nil {
		return err
	}
	key, value, err := ds.decodeDatastoreKeyValue(k, v)
	if err != nil {
		return err
	}
	log.NewLogger("libp2p datastore").Debugf("key=%v value=%v", key, string(value))
	return nil
}

func (ds *notifyingDatastore) decodeDatastoreKeyValue(key dstore.Key, value []byte) (string, []byte, error) {
	buf, err := ds.Batching.Get(key)
	if err == dstore.ErrNotFound {
		return "", nil, nil
	} else if err != nil {
		return "", nil, err
	}

	rec := new(recpb.Record)
	err = proto.Unmarshal(buf, rec)
	if err != nil {
		// Bad data in datastore, log it but don't return an error, we'll just overwrite it
		fmt.Printf("Bad record data stored in datastore with key %s: could not unmarshal record\n", key)
		return "", nil, nil
	}
	return string(rec.Key), rec.Value, nil
}
