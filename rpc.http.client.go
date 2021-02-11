package redwood

import (
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/powerman/rpc-codec/jsonrpc2"

	"github.com/brynbellomy/redwood/types"
)

type HTTPRPCClient struct {
	dialAddr string
	client   *jsonrpc2.Client
	jwt      string
}

func NewHTTPRPCClient(dialAddr string) *HTTPRPCClient {
	var c *HTTPRPCClient
	c = &HTTPRPCClient{
		dialAddr: dialAddr,
		client: jsonrpc2.NewCustomHTTPClient(
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

func (c *HTTPRPCClient) Close() error {
	return c.client.Close()
}

func (c *HTTPRPCClient) Authorize(signingKeypair *SigningKeypair) error {
	req, err := http.NewRequest("AUTHORIZE", c.dialAddr, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
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

	resp, err = httpClient.Do(req)
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

func (c *HTTPRPCClient) Subscribe(args RPCSubscribeArgs) error {
	return c.client.Call("RPC.Subscribe", args, nil)
}

func (c *HTTPRPCClient) NodeAddress() (types.Address, error) {
	var resp RPCNodeAddressResponse
	return resp.Address, c.client.Call("RPC.NodeAddress", nil, &resp)
}

func (c *HTTPRPCClient) AddPeer(args RPCAddPeerArgs) error {
	return c.client.Call("RPC.AddPeer", args, nil)
}

func (c *HTTPRPCClient) KnownStateURIs() ([]string, error) {
	var resp RPCKnownStateURIsResponse
	return resp.StateURIs, c.client.Call("RPC.KnownStateURIs", nil, &resp)
}

func (c *HTTPRPCClient) SendTx(args RPCSendTxArgs) error {
	return c.client.Call("RPC.SendTx", args, nil)
}
