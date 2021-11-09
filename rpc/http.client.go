package rpc

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/powerman/rpc-codec/jsonrpc2"

	"redwood.dev/crypto"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type HTTPClient struct {
	dialAddr   string
	rpcClient  *jsonrpc2.Client
	httpClient *utils.HTTPClient
	jwt        string
}

func NewHTTPClient(dialAddr string) *HTTPClient {
	httpClient := utils.MakeHTTPClient(10*time.Second, 30*time.Second, nil, nil)

	var c *HTTPClient
	c = &HTTPClient{
		dialAddr:   dialAddr,
		httpClient: httpClient,
		rpcClient: jsonrpc2.NewCustomHTTPClient(
			dialAddr,
			jsonrpc2.DoerFunc(func(req *http.Request) (*http.Response, error) {
				if len(c.jwt) > 0 {
					req.Header.Set("Authorization", "Bearer "+c.jwt)
				}
				return httpClient.Do(req)
			}),
		),
	}
	return c
}

func (c *HTTPClient) Close() error {
	return c.rpcClient.Close()
}

func (c *HTTPClient) Authorize(signingKeypair *crypto.SigKeypair) error {
	req, err := http.NewRequest("AUTHORIZE", c.dialAddr, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var challengeResp struct {
		Challenge string `json:"challenge"`
	}
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	if err != nil {
		return err
	}
	challenge, err := hex.DecodeString(challengeResp.Challenge)
	if err != nil {
		return err
	}
	hash := types.HashBytes(challenge)
	sig, err := signingKeypair.SignHash(hash)
	if err != nil {
		return err
	}

	sigHex := hex.EncodeToString(sig)

	req, err = http.NewRequest("AUTHORIZE", c.dialAddr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Challenge", challengeResp.Challenge)
	req.Header.Set("Response", sigHex)

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var jwtResp struct {
		JWT string `json:"jwt"`
	}
	err = json.NewDecoder(resp.Body).Decode(&jwtResp)
	if err != nil {
		return err
	}
	c.jwt = jwtResp.JWT
	return nil
}

func (c *HTTPClient) Subscribe(args SubscribeArgs) error {
	return c.rpcClient.Call("RPC.Subscribe", args, nil)
}

func (c *HTTPClient) Identities() ([]Identity, error) {
	var resp IdentitiesResponse
	return resp.Identities, c.rpcClient.Call("RPC.Identities", nil, &resp)
}

func (c *HTTPClient) NewIdentity(args NewIdentityArgs) error {
	return c.rpcClient.Call("RPC.NewIdentity", args, nil)
}

func (c *HTTPClient) AddPeer(args AddPeerArgs) error {
	return c.rpcClient.Call("RPC.AddPeer", args, nil)
}

func (c *HTTPClient) KnownStateURIs() ([]string, error) {
	var resp KnownStateURIsResponse
	return resp.StateURIs, c.rpcClient.Call("RPC.KnownStateURIs", nil, &resp)
}

func (c *HTTPClient) SendTx(args SendTxArgs) error {
	return c.rpcClient.Call("RPC.SendTx", args, nil)
}

func (c *HTTPClient) StoreBlob(args StoreBlobArgs) (StoreBlobResponse, error) {
	var resp StoreBlobResponse
	return resp, c.rpcClient.Call("RPC.StoreBlob", args, &resp)
}
