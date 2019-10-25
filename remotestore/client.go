package remotestore

import (
	"context"
	"encoding/json"
	"io"

	"github.com/pkg/errors"
	"google.golang.org/grpc"

	rw "github.com/brynbellomy/redwood"
	"github.com/brynbellomy/redwood/ctx"
)

type client struct {
	*ctx.Context
	host       string
	address    rw.Address
	sigprivkey rw.SigningPrivateKey
	client     RemoteStoreClient
	conn       *grpc.ClientConn
	jwt        string
}

// client should conform to rw.Store
var _ rw.Store = (*client)(nil)

func NewClient(host string, address rw.Address, sigprivkey rw.SigningPrivateKey) rw.Store {
	return &client{
		Context:    &ctx.Context{},
		host:       host,
		address:    address,
		client:     nil,
		conn:       nil,
		sigprivkey: sigprivkey,
		jwt:        "",
	}
}

func (c *client) Start() error {
	return c.CtxStart(
		// on startup,
		func() error {
			c.SetLogLabel(c.address.Pretty() + " store:remote")
			c.Infof(0, "opening remote store at %v", c.host)

			// handshaker := newGrpcHandshaker(nil, c.sigprivkey)

			conn, err := grpc.Dial(c.host,
				UnaryClientJWT(c),
				StreamClientJWT(c),
				grpc.WithInsecure(),
			)
			if err != nil {
				c.Errorf("error opening remote store: %v", err)
				return errors.WithStack(err)
			}

			c.client = NewRemoteStoreClient(conn)
			c.conn = conn

			err = c.Authenticate()
			return err
		},
		nil,
		nil,
		// on shutdown
		func() {
			if c.conn != nil {
				c.conn.Close()
			}
		},
	)
}

func (c *client) Authenticate() error {
	authClient, err := c.client.Authenticate(context.TODO())
	if err != nil {
		return err
	}

	msg, err := authClient.Recv()
	if err != nil {
		return err
	}

	challenge := msg.GetAuthenticateChallenge()
	if challenge == nil {
		return errors.New("protocol error")
	}

	sig, err := c.sigprivkey.SignHash(rw.HashBytes(challenge.Challenge))
	if err != nil {
		return err
	}

	err = authClient.Send(&AuthenticateMessage{
		Payload: &AuthenticateMessage_AuthenticateSignature_{&AuthenticateMessage_AuthenticateSignature{
			Signature: sig,
		}},
	})
	if err != nil {
		return err
	}

	msg, err = authClient.Recv()
	if err != nil {
		return err
	}

	resp := msg.GetAuthenticateResponse()
	if resp == nil {
		return errors.New("protocol error")
	}

	c.jwt = resp.Jwt
	return nil
}

func (c *client) AddTx(tx *rw.Tx) error {
	// @@TODO: don't use json
	txBytes, err := json.Marshal(tx)
	if err != nil {
		return err
	}

	//////////////////
	////// encrypt tx bytes
	//////////////////

	txHash := tx.Hash()
	_, err = c.client.AddTx(context.TODO(), &AddTxRequest{TxHash: txHash[:], TxBytes: txBytes})
	if err != nil {
		return err
	}

	c.Infof(0, "wrote tx %v", tx.Hash())
	return nil
}

func (c *client) RemoveTx(txHash rw.Hash) error {
	_, err := c.client.RemoveTx(context.TODO(), &RemoveTxRequest{TxHash: txHash[:]})
	return err
}

func (c *client) FetchTx(txHash rw.Hash) (*rw.Tx, error) {
	resp, err := c.client.FetchTx(context.TODO(), &FetchTxRequest{TxHash: txHash[:]})
	if err != nil {
		return nil, err
	}
	return c.decodeTx(resp.TxBytes)
}

func (c *client) AllTxs() rw.TxIterator {
	txIter := &txIterator{
		ch:       make(chan *rw.Tx),
		chCancel: make(chan struct{}),
	}

	resp, err := c.client.AllTxs(context.TODO(), &AllTxsRequest{})
	if err != nil {
		return &txIterator{err: err}
	}

	go func() {
		defer close(txIter.ch)

		for {
			pkt, err := resp.Recv()
			if err == io.EOF {
				return
			} else if err != nil {
				txIter.err = err
				return
			}

			tx, err := c.decodeTx(pkt.TxBytes)
			if err != nil {
				txIter.err = err
				return
			}

			txIter.ch <- tx
		}
	}()

	return txIter
}

func (c *client) decodeTx(txBytes []byte) (*rw.Tx, error) {
	var tx rw.Tx
	err := json.Unmarshal(txBytes, &tx)
	return &tx, err
}

type txIterator struct {
	ch       chan *rw.Tx
	chCancel chan struct{}
	err      error
}

func (i *txIterator) Next() *rw.Tx {
	select {
	case tx := <-i.ch:
		return tx
	case <-i.chCancel:
		return nil
	}
}

func (i *txIterator) Cancel() {
	close(i.chCancel)
}

func (i *txIterator) Error() error {
	return i.err
}
