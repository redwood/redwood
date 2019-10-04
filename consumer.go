package main

import (
	"context"

	log "github.com/sirupsen/logrus"
)

type consumer struct {
	ID        ID
	transport Transport
	Store     Store
}

func NewConsumer(id ID, port uint, store Store) *consumer {
	// id := RandomID()

	c := &consumer{
		ID:    id,
		Store: store,
	}

	transport, err := NewLibp2pTransport(context.Background(), id, port)
	if err != nil {
		panic(err)
	}

	transport.SetPutHandler(c.onTxReceived)
	transport.SetAckHandler(c.onAckReceived)

	c.transport = transport

	return c
}

func (c *consumer) onTxReceived(tx Tx) {
	log.Infof("[consumer %v] tx %v received", c.ID.Pretty(), tx.ID)

	err := c.Store.AddTx(tx)
	if err != nil {
		panic(err)
	}
}

func (c *consumer) onAckReceived(url string, id ID) {
	log.Infof("[consumer %v] ack received", c.ID.Pretty(), id)
}

func (c *consumer) AddPeer(ctx context.Context, multiaddrString string) error {
	return c.transport.AddPeer(ctx, multiaddrString)
}

func (c *consumer) Subscribe(ctx context.Context, url string) error {
	return c.transport.Subscribe(ctx, url)
}

func (c *consumer) AddTx(ctx context.Context, tx Tx) error {
	err := c.Store.AddTx(tx)
	if err != nil {
		return err
	}

	err = c.transport.Put(ctx, tx)
	if err != nil {
		return err
	}
	return nil
}
