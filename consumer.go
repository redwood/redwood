package redwood

import (
	"context"

	"github.com/plan-systems/plan-core/tools/ctx"
)

type consumer struct {
	ctx.Context

	ID        ID
	Port      uint
	Transport Transport
	Store     Store
}

func NewConsumer(id ID, port uint, store Store) *consumer {
	c := &consumer{
		ID:    id,
		Port:  port,
		Store: store,
	}

	c.Startup()

	return c
}

func (c *consumer) Startup() error {

	err := c.CtxStart(
		c.ctxStartup,
		nil,
		nil,
		c.ctxStopping,
	)

	return err
}

func (c *consumer) ctxStartup() error {

	c.SetLogLabel("consumer" + c.ID.Pretty()[:4])
	c.Infof(0, "opening libp2p on port %v", c.Port)
	transport, err := NewLibp2pTransport(c.Ctx, c.ID, c.Port)
	if err != nil {
		return err
	}

	transport.SetPutHandler(c.onTxReceived)
	transport.SetAckHandler(c.onAckReceived)

	c.Transport = transport

	return nil
}

func (c *consumer) ctxStopping() {

	// No op since c.Ctx will cancel as this ctx completes stopping

}

func (c *consumer) onTxReceived(tx Tx) {
	c.Infof(0, "tx %v received", c.ID.Pretty())

	err := c.Store.AddTx(tx)
	if err != nil {
		panic(err)
	}
}

func (c *consumer) onAckReceived(url string, id ID) {
	c.Info(0, "ack received for ", url)
}

func (c *consumer) AddPeer(ctx context.Context, multiaddrString string) error {
	return c.Transport.AddPeer(ctx, multiaddrString)
}

func (c *consumer) Subscribe(ctx context.Context, url string) error {
	return c.Transport.Subscribe(ctx, url)
}

func (c *consumer) AddTx(ctx context.Context, tx Tx) error {

	c.Info(0, "adding tx ", tx.ID.Pretty())
	err := c.Store.AddTx(tx)
	if err != nil {
		return err
	}

	err = c.Transport.Put(ctx, tx)
	if err != nil {
		return err
	}
	return nil
}
