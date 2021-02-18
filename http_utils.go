package redwood

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

// Creates an *http.Request representing the given Tx that follows the Braid-HTTP
// specification for sending transactions/patches to peers. If the transaction is
// public, the `senderEncKeypair` and `recipientEncPubkey` parameters may be nil.
func PutRequestFromTx(
	requestContext context.Context,
	tx *Tx,
	dialAddr string,
	senderEncKeypair *EncryptingKeypair,
	recipientEncPubkey EncryptingPublicKey,
) (*http.Request, error) {
	if tx.IsPrivate() {
		if recipientEncPubkey == nil {
			return nil, errors.New("no encrypting pubkey provided")
		}

		marshalledTx, err := json.Marshal(tx)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		encryptedTxBytes, err := senderEncKeypair.SealMessageFor(recipientEncPubkey, marshalledTx)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		msg, err := json.Marshal(EncryptedTx{
			TxID:             tx.ID,
			EncryptedPayload: encryptedTxBytes,
			SenderPublicKey:  senderEncKeypair.EncryptingPublicKey.Bytes(),
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}

		req, err := http.NewRequestWithContext(requestContext, "PUT", dialAddr, bytes.NewReader(msg))
		if err != nil {
			return nil, errors.WithStack(err)
		}
		req.Header.Set("Private", "true")

		return req, nil
	}

	var parentStrs []string
	for _, parent := range tx.Parents {
		parentStrs = append(parentStrs, parent.Hex())
	}

	var body bytes.Buffer
	for _, patch := range tx.Patches {
		_, err := body.Write([]byte(patch.String() + "\n"))
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	req, err := http.NewRequest("PUT", dialAddr, &body)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	req.Header.Set("Version", tx.ID.Hex())
	req.Header.Set("State-URI", tx.StateURI)
	req.Header.Set("Signature", tx.Sig.Hex())
	req.Header.Set("Parents", strings.Join(parentStrs, ","))
	if tx.Checkpoint {
		req.Header.Set("Checkpoint", "true")
	}
	return req, nil
}
