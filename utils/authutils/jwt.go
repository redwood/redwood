package authutils

import (
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"

	"redwood.dev/errors"
	"redwood.dev/types"
	. "redwood.dev/utils/generics"
)

type Ucan struct {
	Addresses Set[types.Address]
	// Capabilities Set[C]
	IssuedAt time.Time
}

func (u Ucan) SignedString(jwtSecret []byte) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"addresses": Map(u.Addresses.Slice(), func(a types.Address) string { return a.Hex() }),
		// "capabilities": capabilities,
		"iat": u.IssuedAt.UTC().Unix(),
	})
	return token.SignedString(jwtSecret)
}

func (u *Ucan) Parse(str string, jwtSecret []byte) error {
	claims, exists, err := ParseJWT(str, jwtSecret)
	if err != nil {
		return err
	} else if !exists {
		return errors.Err404
	}

	if u == nil {
		*u = Ucan{}
	}

	addrsHex, ok := claims["addresses"].([]any)
	if ok {
		addrs, _ := MapWithError(addrsHex, func(addr any) (types.Address, error) {
			addrStr, ok := addr.(string)
			if !ok {
				return types.Address{}, errors.New("")
			}
			return types.AddressFromHex(addrStr)
		})
		u.Addresses = NewSet(addrs)
	}

	// caps, ok := claims["capabilities"].([]any)
	// if ok {
	// 	capabilities, _ := MapWithError(caps, func(c any) (C, error) {
	// 		cap, ok := c.(C)
	// 		if !ok {
	// 			return cap, errors.New("could not cast JSON type to Capability type")
	// 		}
	// 		return cap, nil
	// 	})
	// 	u.Capabilities = NewSet(capabilities)
	// }

	iat, ok := claims["iat"].(float64)
	if ok {
		u.IssuedAt = time.Unix(int64(iat), 0)
	}
	return nil
}

func ParseJWT(ucan string, jwtSecret []byte) (jwt.MapClaims, bool, error) {
	if strings.HasPrefix(ucan, "Bearer ") {
		ucan = ucan[len("Bearer "):]
	}
	ucan = strings.TrimSpace(ucan)

	if ucan == "" {
		return nil, false, nil
	}

	token, err := jwt.Parse(ucan, func(token *jwt.Token) (interface{}, error) {
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
		return nil, false, errors.Errorf("invalid jwt")
	}
	return claims, true, nil
}
