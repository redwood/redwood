package redwood

import (
	"context"

	"github.com/plan-systems/plan-core/tools/ctx"
)

type consumer struct {
	ctx.Context

	// ID        ID
	Port      uint
	Transport Transport
	Store     Store
	privkey   *privateKey
}

func NewConsumer(privkey *privateKey, port uint, store Store) (*consumer, error) {
	c := &consumer{
		// ID:      id,
		Port:    port,
		Store:   store,
		privkey: privkey,
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

func (c *consumer) Address() Address {
	return c.privkey.PublicKey().Address()
}

func (c *consumer) ctxStartup() error {
	c.SetLogLabel(c.Address().Pretty() + " consumer")
	c.Infof(0, "opening libp2p on port %v", c.Port)
	transport, err := NewLibp2pTransport(c.Ctx, c.Address().Pretty(), c.Port)
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

	hash, err := tx.Hash()
	if err != nil {
		return err
	}

	sig, err := c.privkey.SignHash(hash)
	if err != nil {
		return err
	}

	tx.Sig = sig

	err = c.Store.AddTx(&tx)
	if err != nil {
		return err
	}

	err = c.Transport.Put(c.Ctx, tx)
	if err != nil {
		return err
	}
	return nil
}
