package vault

import (
	"bytes"
	"context"
	"io"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/types"
)

type Client struct {
	log.Logger
	conn     *grpc.ClientConn
	client   VaultRPCClient
	host     string
	identity identity.Identity
	jwt      string
	store    ClientStore
}

func NewClient(host string, identity identity.Identity, store ClientStore) *Client {
	return &Client{
		Logger:   log.NewLogger("vault client"),
		host:     host,
		identity: identity,
		store:    store,
	}
}

func (c *Client) Dial(ctx context.Context) error {
	conn, err := grpc.Dial(c.host, grpc.WithInsecure())
	if err != nil {
		return err
	}
	c.conn = conn
	c.client = NewVaultRPCClient(conn)
	return c.authorize(ctx)
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Host() string {
	return c.host
}

func (c *Client) Identity() identity.Identity {
	return c.identity
}

func (c *Client) authorize(ctx context.Context) error {
	stream, err := c.client.Authorize(ctx)
	if err != nil {
		return err
	}

	msg, err := stream.Recv()
	if err != nil {
		return err
	}

	challenge := msg.GetChallenge()
	if challenge == nil {
		return errors.Errorf("protocol error")
	}

	sig, err := c.identity.SignHash(types.HashBytes(challenge.Challenge))
	if err != nil {
		return err
	}

	err = stream.Send(&AuthorizeMsg{
		Msg: &AuthorizeMsg_SignedChallenge_{
			SignedChallenge: &AuthorizeMsg_SignedChallenge{
				Signature: crypto.Signature(sig),
			},
		},
	})
	if err != nil {
		return err
	}

	msg, err = stream.Recv()
	if err != nil {
		return err
	}

	resp := msg.GetResponse()
	if resp == nil {
		return errors.Errorf("protocol error")
	}

	c.jwt = resp.Token
	return nil
}

func (c *Client) doWithReauth(ctx context.Context, fn func() error) error {
	err := fn()
	if err != nil && status.Convert(err).Code() == codes.Unauthenticated {
		err = c.authorize(ctx)
		if err != nil {
			return err
		}
		err = fn()
		return err

	} else if err != nil {
		return err
	}
	return nil
}

func (c *Client) Items(ctx context.Context, collectionID string, oldestMtime time.Time) ([]string, error) {
	var itemIDs []string
	err := c.doWithReauth(ctx, func() error {
		resp, err := c.client.Items(ctx, &ItemsReq{JWT: c.jwt, CollectionID: collectionID, OldestMtime: uint64(oldestMtime.UTC().Unix())})
		if err != nil {
			return err
		}
		itemIDs = resp.ItemIDs
		return nil
	})
	return itemIDs, err
}

func (c *Client) LatestItems(ctx context.Context, collectionID string) ([]string, error) {
	mtime := c.store.LatestMtimeForVaultAndCollection(c.host, collectionID)
	return c.Items(ctx, collectionID, mtime)
}

type ClientItem struct {
	io.ReadCloser
	Host         string
	CollectionID string
	ItemID       string
	Mtime        time.Time
}

func (c *Client) FetchLatestFromCollection(ctx context.Context, collectionID string) (<-chan ClientItem, error) {
	itemIDs, err := c.LatestItems(ctx, collectionID)
	if err != nil {
		return nil, err
	}

	ch := make(chan ClientItem)
	go func() {
		defer close(ch)

		for _, itemID := range itemIDs {
			item, err := c.Fetch(ctx, collectionID, itemID)
			if err != nil {
				c.Errorw("failed to fetch item from vault", "host", c.host, "collection", collectionID, "item", itemID, "err", err)
				continue
			}

			select {
			case <-ctx.Done():
				return
			case ch <- item:
			}
		}
	}()

	return ch, nil
}

func (c *Client) Fetch(ctx context.Context, collectionID, itemID string) (ClientItem, error) {
	var stream VaultRPC_FetchClient

	err := c.doWithReauth(ctx, func() (err error) {
		stream, err = c.client.Fetch(ctx, &FetchReq{
			JWT:          c.jwt,
			CollectionID: collectionID,
			ItemID:       itemID,
		})
		return
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ClientItem{}, errors.Err404
		}
		return ClientItem{}, err
	}

	msg, err := stream.Recv()
	if err != nil {
		return ClientItem{}, err
	}

	reader, writer := io.Pipe()

	go func() {
		var msg *FetchResp
		var err error
		defer func() { writer.CloseWithError(err) }()

		r := bytes.NewReader(nil)
		for {
			msg, err = stream.Recv()
			if err != nil {
				return
			}

			r.Reset(msg.Data)
			_, err = io.Copy(writer, r)
			if err != nil {
				return
			}
			if msg.End {
				return
			}
		}
	}()

	return ClientItem{reader, c.Host(), collectionID, itemID, time.Unix(int64(msg.Mtime), 0)}, nil
}

func (c *Client) Store(ctx context.Context, collectionID, itemID string, reader io.Reader) error {
	var stream VaultRPC_StoreClient

	err := c.doWithReauth(ctx, func() (err error) {
		stream, err = c.client.Store(ctx)
		return
	})
	if err != nil {
		return err
	}

	err = stream.Send(&StoreReq{
		Msg: &StoreReq_Header_{
			Header: &StoreReq_Header{
				JWT:          c.jwt,
				CollectionID: collectionID,
				ItemID:       itemID,
			},
		},
	})
	if err != nil {
		return err
	}

	_, err = io.Copy(StoreClientWriter{stream}, reader)
	if err != nil {
		return err
	}

	err = stream.Send(&StoreReq{
		Msg: &StoreReq_Payload_{
			Payload: &StoreReq_Payload{
				End: true,
			},
		},
	})
	if err != nil {
		return err
	}

	_, err = stream.CloseAndRecv()
	return err
}

type StoreClientWriter struct {
	VaultRPC_StoreClient
}

func (w StoreClientWriter) Write(bs []byte) (int, error) {
	err := VaultRPC_StoreClient(w).Send(&StoreReq{
		Msg: &StoreReq_Payload_{
			Payload: &StoreReq_Payload{
				Data: bs,
			},
		},
	})
	return len(bs), err
}

func (c *Client) Delete(ctx context.Context, collectionID, itemID string) error {
	_, err := c.client.Delete(ctx, &DeleteReq{
		JWT:          c.jwt,
		CollectionID: collectionID,
		ItemID:       itemID,
	})
	return err
}

func (c *Client) SetUserCapabilities(ctx context.Context, userAddr types.Address, capabilities []Capability) error {
	return c.doWithReauth(ctx, func() error {
		_, err := c.client.SetUserCapabilities(ctx, &SetUserCapabilitiesReq{
			JWT:          c.jwt,
			Address:      userAddr,
			Capabilities: capabilities,
		})
		return err
	})
}
