package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

func main() {
	ctx := context.Background()

	var id1 ID
	var id2 ID

	copy(id1[:], []byte("oneoneoneoneone"))
	copy(id2[:], []byte("twotwotwotwotwo"))

	c1 := NewConsumer(id1, 9123)
	c2 := NewConsumer(id2, 9124)

	c2.AddPeer(ctx, "/ip4/0.0.0.0/tcp/9123/p2p/16Uiu2HAkytaj6U3nnmu1mBVQdpAzzg9DdPtP4FX4xN7FhAkG2St5")

	err := c2.Subscribe(ctx, "axon.science")
	if err != nil {
		panic(err)
	}

	time.Sleep(2 * time.Second)
	log.Infof("adding 2 txs")

	c1.AddTx(ctx, "axon.science", Tx{
		ID: RandomID(),
		Patches: []Patch{
			{
				Keys: []string{"foo"},
				Val:  123,
			},
		},
	})

	c1.AddTx(ctx, "axon.science", Tx{
		ID: RandomID(),
		Patches: []Patch{
			{
				Keys: []string{"foo", "bar", "baz"},
				Val:  123,
			},
		},
	})

	// Block forever
	ch := make(chan struct{})
	<-ch
}
