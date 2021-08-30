package remotetxstore

import (
	"context"
	"encoding/json"
	"io"

	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"redwood.dev/crypto"
	"redwood.dev/log"
	"redwood.dev/tree"
	"redwood.dev/types"
)

type client struct {
	log.Logger
	host       string
	address    types.Address
	sigprivkey crypto.SigningPrivateKey
	client     RemoteStoreClient
	conn       *grpc.ClientConn
	jwt        string
}

// client should conform to tree.TxStore
var _ tree.TxStore = (*client)(nil)

func NewClient(host string, address types.Address, sigprivkey crypto.SigningPrivateKey) *client {
	return &client{
		Logger:     log.NewLogger("vault client"),
		host:       host,
		address:    address,
		client:     nil,
		conn:       nil,
		sigprivkey: sigprivkey,
		jwt:        "",
	}
}

func (c *client) Start() error {
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
}

func (c *client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
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

	sig, err := c.sigprivkey.SignHash(types.HashBytes(challenge.Challenge))
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

func (c *client) AddTx(tx *tree.Tx) error {
	// @@TODO: don't use json
	txBytes, err := json.Marshal(tx)
	if err != nil {
		return err
	}

	//////////////////
	////// encrypt tx bytes
	//////////////////

	txHash := tx.Hash()
	_, err = c.client.AddMessage(context.TODO(), &AddMessageRequest{Id: txHash[:], Data: txBytes})
	if err != nil {
		return err
	}

	c.Infof(0, "wrote tx %v", tx.Hash())
	return nil
}

func (c *client) AllTxsForStateURI(stateURI string, fromTxIDs types.IDSet) tree.TxIterator {
	txIter := &txIterator{
		ch:       make(chan *tree.Tx),
		chCancel: make(chan struct{}),
	}

	resp, err := c.client.AllMessages(context.TODO(), &AllMessagesRequest{})
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

			tx, err := c.decodeTx(pkt.Data)
			if err != nil {
				txIter.err = err
				return
			}

			txIter.ch <- tx
		}
	}()

	return txIter
}

func (c *client) FetchTx(stateURI string, txID types.ID) (*tree.Tx, error) { panic("unimplemented") }
func (c *client) TxExists(stateURI string, txID types.ID) (bool, error)    { panic("unimplemented") }
func (c *client) RemoveTx(stateURI string, txID types.ID) error            { panic("unimplemented") }
func (c *client) KnownStateURIs() ([]string, error)                        { panic("unimplemented") }
func (c *client) MarkLeaf(stateURI string, txID types.ID) error            { panic("unimplemented") }
func (c *client) UnmarkLeaf(stateURI string, txID types.ID) error          { panic("unimplemented") }
func (c *client) Leaves(stateURI string) ([]types.ID, error)               { panic("unimplemented") }

func (c *client) decodeTx(txBytes []byte) (*tree.Tx, error) {
	var tx tree.Tx
	err := json.Unmarshal(txBytes, &tx)
	return &tx, err
}

type txIterator struct {
	ch       chan *tree.Tx
	chCancel chan struct{}
	err      error
}

func (i *txIterator) Next() *tree.Tx {
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
