package utils

import (
	"crypto/tls"
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/rs/cors"

	"redwood.dev/errors"
)

type HTTPClient struct {
	http.Client
	chStop chan struct{}
}

func MakeHTTPClient(requestTimeout, reapIdleConnsInterval time.Duration, cookieJar http.CookieJar, tlsCerts []tls.Certificate) *HTTPClient {
	c := http.Client{
		Timeout: requestTimeout,
		Jar:     cookieJar,
	}

	c.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS13,
			MaxVersion:         tls.VersionTLS13,
			Certificates:       tlsCerts,
			ClientAuth:         tls.RequestClientCert,
			InsecureSkipVerify: true,
		},
	}

	chStop := make(chan struct{})

	if reapIdleConnsInterval > 0 {
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
	}

	return &HTTPClient{c, chStop}
}

func (c HTTPClient) Close() {
	close(c.chStop)
}

var unmarshalRequestRegexp = regexp.MustCompile(`(header|query|url):"([^"]+)"`)
var stringType = reflect.TypeOf("")

func UnmarshalHTTPRequest(into interface{}, r *http.Request) error {
	rval := reflect.ValueOf(into).Elem()

	for i := 0; i < rval.Type().NumField(); i++ {
		field := rval.Type().Field(i)
		matches := unmarshalRequestRegexp.FindAllStringSubmatch(string(field.Tag), -1)
		var found bool
		for _, match := range matches {
			source := match[1]
			var name string
			if len(match) > 2 {
				name = match[2]
			}

			var value string
			switch source {
			case "header":
				value = r.Header.Get(name)
			case "query":
				value = r.URL.Query().Get(name)
			case "url":
				value = r.URL.Path
			default:
				panic("invariant violation")
			}

			if value == "" {
				continue
			}

			fieldVal := rval.Field(i)

			var err error
			if fieldVal.Kind() == reflect.Ptr {
				err = unmarshalHTTPField(field.Name, value, fieldVal)
			} else if fieldVal.CanAddr() {
				err = unmarshalHTTPField(field.Name, value, fieldVal.Addr())
			} else {
				return errors.Errorf("cannot unmarshal into struct field '%v'", field.Name)
			}
			if err != nil {
				return err
			}
			found = true
			break
		}
		if !found {
			if field.Tag.Get("required") != "" {
				return errors.Errorf("missing request field '%v'", field.Name)
			}
		}
	}
	return nil
}

var unmarshalResponseRegexp = regexp.MustCompile(`(header):"([^"]+)"`)

func UnmarshalHTTPResponse(into interface{}, r *http.Response) error {
	rval := reflect.ValueOf(into).Elem()

	for i := 0; i < rval.Type().NumField(); i++ {
		field := rval.Type().Field(i)
		matches := unmarshalRequestRegexp.FindAllStringSubmatch(string(field.Tag), -1)
		var found bool
		for _, match := range matches {
			source := match[1]
			name := match[2]

			var value string
			switch source {
			case "header":
				value = r.Header.Get(name)
			default:
				panic("invariant violation")
			}

			if value == "" {
				continue
			}

			fieldVal := rval.Field(i)

			var err error
			if fieldVal.Kind() == reflect.Ptr {
				err = unmarshalHTTPField(field.Name, value, fieldVal)
			} else if fieldVal.CanAddr() {
				err = unmarshalHTTPField(field.Name, value, fieldVal.Addr())
			} else {
				return errors.Errorf("cannot unmarshal into struct field '%v'", field.Name)
			}
			if err != nil {
				return err
			}
			found = true
			break
		}
		if !found {
			if field.Tag.Get("required") != "" {
				return errors.Errorf("missing request field '%v'", field.Name)
			}
		}
	}
	return nil
}

func unmarshalHTTPField(name, value string, fieldVal reflect.Value) error {
	if as, is := fieldVal.Interface().(encoding.TextUnmarshaler); is {
		return as.UnmarshalText([]byte(value))

	} else if fieldVal.Type().Elem().ConvertibleTo(stringType) {
		fieldVal.Elem().Set(reflect.ValueOf(value).Convert(fieldVal.Type().Elem()))
		return nil

	} else {
		switch fieldVal.Kind() {
		case reflect.Int:
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			fieldVal.Set(reflect.ValueOf(int(n)).Convert(fieldVal.Type().Elem()))
		case reflect.Int8:
			n, err := strconv.ParseInt(value, 10, 8)
			if err != nil {
				return err
			}
			fieldVal.Set(reflect.ValueOf(int8(n)).Convert(fieldVal.Type().Elem()))
		case reflect.Int16:
			n, err := strconv.ParseInt(value, 10, 16)
			if err != nil {
				return err
			}
			fieldVal.Set(reflect.ValueOf(int16(n)).Convert(fieldVal.Type().Elem()))
		case reflect.Int32:
			n, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return err
			}
			fieldVal.Set(reflect.ValueOf(int32(n)).Convert(fieldVal.Type().Elem()))
		case reflect.Int64:
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			fieldVal.Set(reflect.ValueOf(int64(n)).Convert(fieldVal.Type().Elem()))

		case reflect.Uint:
			n, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return err
			}
			fieldVal.Set(reflect.ValueOf(uint(n)).Convert(fieldVal.Type().Elem()))
		case reflect.Uint8:
			n, err := strconv.ParseUint(value, 10, 8)
			if err != nil {
				return err
			}
			fieldVal.Set(reflect.ValueOf(uint8(n)).Convert(fieldVal.Type().Elem()))
		case reflect.Uint16:
			n, err := strconv.ParseUint(value, 10, 16)
			if err != nil {
				return err
			}
			fieldVal.Set(reflect.ValueOf(uint16(n)).Convert(fieldVal.Type().Elem()))
		case reflect.Uint32:
			n, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return err
			}
			fieldVal.Set(reflect.ValueOf(uint32(n)).Convert(fieldVal.Type().Elem()))
		case reflect.Uint64:
			n, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return err
			}
			fieldVal.Set(reflect.ValueOf(uint64(n)).Convert(fieldVal.Type().Elem()))

		case reflect.Bool:
			b, err := strconv.ParseBool(value)
			if err != nil {
				return err
			}
			fieldVal.Set(reflect.ValueOf(b).Convert(fieldVal.Type().Elem()))

		default:
			panic(fmt.Sprintf(`cannot unmarshal http.Request into struct field "%v" of type %T`, name, fieldVal.Type()))
		}
	}
	return nil
}

func ParseJWT(authHeader string, jwtSecret []byte) (jwt.MapClaims, bool, error) {
	if authHeader == "" {
		return nil, false, nil
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, false, errors.Errorf("bad Authorization header")
	}

	jwtToken := strings.TrimSpace(authHeader[len("Bearer "):])

	token, err := jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, false, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, false, errors.Errorf("invalid jwt token")
	}
	return claims, true, nil
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
