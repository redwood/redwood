package redwood

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

type HTTPRPCClient struct {
	dialAddr   string
	rpcClient  *jsonrpc2.Client
	httpClient *utils.HTTPClient
	jwt        string
}

func NewHTTPRPCClient(dialAddr string) *HTTPRPCClient {
	httpClient := utils.MakeHTTPClient(10*time.Second, 30*time.Second)

	var c *HTTPRPCClient
	c = &HTTPRPCClient{
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

func (c *HTTPRPCClient) Close() error {
	return c.rpcClient.Close()
}

func (c *HTTPRPCClient) Authorize(signingKeypair *crypto.SigningKeypair) error {
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

func (c *HTTPRPCClient) Subscribe(args RPCSubscribeArgs) error {
	return c.rpcClient.Call("RPC.Subscribe", args, nil)
}

func (c *HTTPRPCClient) Identities() ([]RPCIdentity, error) {
	var resp RPCIdentitiesResponse
	return resp.Identities, c.client.Call("RPC.Identities", nil, &resp)
}

func (c *HTTPRPCClient) NewIdentity(args RPCNewIdentityArgs) error {
	return c.client.Call("RPC.NewIdentity", args, nil)
}

func (c *HTTPRPCClient) AddPeer(args RPCAddPeerArgs) error {
	return c.rpcClient.Call("RPC.AddPeer", args, nil)
}

func (c *HTTPRPCClient) KnownStateURIs() ([]string, error) {
	var resp RPCKnownStateURIsResponse
	return resp.StateURIs, c.rpcClient.Call("RPC.KnownStateURIs", nil, &resp)
}

func (c *HTTPRPCClient) SendTx(args RPCSendTxArgs) error {
	return c.rpcClient.Call("RPC.SendTx", args, nil)
}
