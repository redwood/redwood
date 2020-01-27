package demoutils

import (
	"io/ioutil"

	rw "github.com/brynbellomy/redwood"
)

func MakeHost(signingKeypairHex string, port uint, cookieSecretStr, tlsCertFilename, tlsKeyFilename string) rw.Host {
	signingKeypair, err := rw.SigningKeypairFromHex(signingKeypairHex)
	if err != nil {
		panic(err)
	}

	txDBRoot, err := ioutil.TempDir("", "redwood-txs-")
	if err != nil {
		panic(err)
	}
	stateDBRoot, err := ioutil.TempDir("", "redwood-state-")
	if err != nil {
		panic(err)
	}
	refStoreRoot, err := ioutil.TempDir("", "redwood-refs-")
	if err != nil {
		panic(err)
	}

	encryptingKeypair, err := rw.GenerateEncryptingKeypair()
	if err != nil {
		panic(err)
	}

	txStore := rw.NewBadgerTxStore(txDBRoot, signingKeypair.Address())
	// txStore := remotestore.NewClient("0.0.0.0:4567", signingKeypair.Address(), signingKeypair.SigningPrivateKey)
	refStore := rw.NewRefStore(refStoreRoot)
	peerStore := rw.NewPeerStore(signingKeypair.Address())
	metacontroller := rw.NewMetacontroller(signingKeypair.Address(), stateDBRoot, txStore, refStore)

	p2ptransport, err := rw.NewLibp2pTransport(signingKeypair.Address(), port, metacontroller, refStore, peerStore)
	if err != nil {
		panic(err)
	}

	var cookieSecret [32]byte
	copy(cookieSecret[:], []byte(cookieSecretStr))
	httptransport, err := rw.NewHTTPTransport(signingKeypair.Address(), port+1, "localhost:21231", metacontroller, refStore, peerStore, signingKeypair, cookieSecret, tlsCertFilename, tlsKeyFilename)
	if err != nil {
		panic(err)
	}

	transports := []rw.Transport{p2ptransport, httptransport}

	h, err := rw.NewHost(signingKeypair, encryptingKeypair, port, transports, metacontroller, refStore, peerStore)
	if err != nil {
		panic(err)
	}

	return h
}
