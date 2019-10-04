package redwood

import (
	"context"
	"io"
)

type Transport interface {
	AddPeer(ctx context.Context, multiaddrString string) error

	Subscribe(ctx context.Context, url string) error
	Ack(ctx context.Context, url string, versionID ID) error
	Put(ctx context.Context, tx Tx) error

	SetAckHandler(handler AckHandler)
	SetPutHandler(handler PutHandler)
}

type AckHandler func(url string, version ID)
type PutHandler func(tx Tx)

type subscriptionOut struct {
	io.ReadCloser
	chDone chan struct{}
}
