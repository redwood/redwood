package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"redwood.dev/rpc"
	"redwood.dev/state"
	"redwood.dev/tree"
	"redwood.dev/utils"
)

type M = map[string]interface{}

const stateURI = "foo.bar/baz"

var txsSent int
var mu2 sync.Mutex

func main() {
	c := rpc.NewHTTPClient("http://127.0.0.1:8081")

	sendTx(c, tree.GenesisTxID, nil, M{
		"Merge-Type": M{
			"Content-Type": "resolver/dumb",
			"value":        M{},
		},
		"Validator": M{
			"Content-Type": "validator/permissions",
			"value": M{
				"*": M{
					"^.*$": M{
						"write": true,
					},
				},
			},
		},
		"users": M{},
	})

	m := make(map[int]int)
	mu := sync.Mutex{}

	for i := 0; i < 100; i++ {
		i := i
		go func() {
			c := http.Client{}
			req, err := http.NewRequest("GET", "http://127.0.0.1:8080", nil)
			if err != nil {
				panic(err)
			}
			req.Header.Set("Subscribe", "states")
			req.Header.Set("State-URI", stateURI)
			resp, err := c.Do(req)
			if err != nil {
				panic(err)
			}
			defer resp.Body.Close()

			r := bufio.NewScanner(resp.Body)
			for r.Scan() {
				if r.Text() == "" {
					continue
				}
				mu.Lock()
				x := m[i]
				m[i] = x + 1
				mu.Unlock()
			}
		}()
	}

	for i := 0; i < 10; i++ {
		go func() {
			c := rpc.NewHTTPClient("http://127.0.0.1:8081")
			kp := state.Keypath(utils.RandomString(16))
			for {
				sendTx(c, state.RandomVersion(), kp, M{utils.RandomString(16): utils.RandomString(16)})
				time.Sleep(1 * time.Second)
			}
		}()
	}

	select {}
}

func sendTx(c *rpc.HTTPClient, txID state.Version, kp state.Keypath, value interface{}) {
	valueBytes, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}

	err = c.SendTx(rpc.SendTxArgs{
		Tx: tree.Tx{
			StateURI: stateURI,
			ID:       txID,
			Patches:  []tree.Patch{{Keypath: kp, ValueJSON: valueBytes}},
		},
	})
	if err != nil {
		panic(err)
	}
	mu2.Lock()
	txsSent++
	mu2.Unlock()
}
