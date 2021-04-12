package utils

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/cors"
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

func RespondJSON(resp http.ResponseWriter, data interface{}) {
	resp.Header().Add("Content-Type", "application/json")

	err := json.NewEncoder(resp).Encode(data)
	if err != nil {
		panic(err)
	}
}

func UnrestrictedCors(handler http.Handler) http.Handler {
	return cors.New(cors.Options{
		AllowOriginFunc:  func(string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "AUTHORIZE", "SUBSCRIBE", "ACK", "OPTIONS", "HEAD"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"*"},
		AllowCredentials: true,
	}).Handler(handler)
}

func SniffContentType(filename string, data io.Reader) (string, error) {
	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)

	_, err := data.Read(buffer)
	if err != nil {
		return "", err
	}

	// Use the net/http package's handy DectectContentType function. Always returns a valid
	// content-type by returning "application/octet-stream" if no others seemed to match.
	contentType := http.DetectContentType(buffer)

	// If we got an ambiguous result, check the file extension
	if contentType == "application/octet-stream" {
		contentType = GuessContentTypeFromFilename(filename)
	}
	return contentType, nil
}

func GuessContentTypeFromFilename(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) > 1 {
		ext := strings.ToLower(parts[len(parts)-1])
		switch ext {
		case "txt":
			return "text/plain"
		case "html":
			return "text/html"
		case "js":
			return "application/js"
		case "json":
			return "application/json"
		case "png":
			return "image/png"
		case "jpg", "jpeg":
			return "image/jpeg"
		}
	}
	return "application/octet-stream"
}
