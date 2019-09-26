package main

import (
	"context"
)

type consumer struct {
	host  Host
	store Store
}

func (c *consumer) AddTx(ctx context.Context, tx Tx) error {
	err := c.store.AddTx(tx)
	if err != nil {
		return err
	}

	err = c.host.BroadcastTx(tx)
	if err != nil {
		return err
	}
}
