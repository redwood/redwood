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

func NewConsumer(id ID, port uint, store Store) (*consumer, error) {
	c := &consumer{
		ID:    id,
		Port:  port,
		Store: store,
	}

	err := c.Startup()

	return c, err
}

func (c *consumer) Startup() error {
	return c.CtxStart(
		c.ctxStartup,
		nil,
		nil,
		c.ctxStopping,
	)
}

func (c *consumer) ctxStartup() error {
	c.SetLogLabel(c.ID.Pretty()[:4] + " consumer")
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
	c.Infof(0, "tx %v received", tx.ID.Pretty())

	err := c.Store.AddTx(&tx)
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

func (c *consumer) AddTx(tx Tx) error {
	c.Info(0, "adding tx ", tx.ID.Pretty())
	err := c.Store.AddTx(&tx)
	if err != nil {
		return err
	}

	err = c.Transport.Put(c.Ctx, tx)
	if err != nil {
		return err
	}
	return nil
}
