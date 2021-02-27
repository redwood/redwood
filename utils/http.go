package utils

import (
	"net/http"
	"time"
)

type HTTPClient struct {
	http.Client
	chStop chan struct{}
}

func MakeHTTPClient(requestTimeout, reapIdleConnsInterval time.Duration) *HTTPClient {
	var c http.Client
	c.Timeout = requestTimeout

	chStop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(reapIdleConnsInterval)
		defer ticker.Stop()
		defer c.CloseIdleConnections()

		for {
			select {
			case <-ticker.C:
				c.CloseIdleConnections()
			case <-chStop:
				return
			}
		}
	}()

	return &HTTPClient{c, chStop}
}

func (c HTTPClient) Close() {
	close(c.chStop)
}
