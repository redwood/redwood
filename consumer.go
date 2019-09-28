package main

import (
	"context"

	log "github.com/sirupsen/logrus"
)

type consumer struct {
	ID    ID
	host  Host
	store Store
}

func NewConsumer(id ID, port uint) *consumer {
	// id := RandomID()

	c := &consumer{
		ID:    id,
		store: NewStore(id),
	}

	transport, err := NewLibp2pTransport(context.Background(), id, port)
	if err != nil {
		panic(err)
	}

	c.host = NewHost(
		id,
		transport,
		c.onTxReceived,
		c.onAckReceived,
	)

	return c
}

func (c *consumer) onTxReceived(url string, tx Tx) {
	log.Infof("[consumer %v] tx %v received", c.ID.Pretty(), tx.ID)

	err := c.store.AddTx(tx)
	if err != nil {
		panic(err)
	}
}

func (c *consumer) onAckReceived(url string, id ID) {
	log.Infof("[consumer %v] ack received", c.ID.Pretty(), id)
}

func (c *consumer) AddPeer(ctx context.Context, multiaddrString string) error {
	return c.host.AddPeer(ctx, multiaddrString)
}

func (c *consumer) Subscribe(ctx context.Context, url string) error {
	return c.host.Subscribe(ctx, url)
}

func (c *consumer) AddTx(ctx context.Context, url string, tx Tx) error {
	err := c.store.AddTx(tx)
	if err != nil {
		return err
	}

	err = c.host.BroadcastTx(ctx, url, tx)
	if err != nil {
		return err
	}
	return nil
}
