package main

import (
	"flag"
	"os"
	"path/filepath"

	rw "github.com/brynbellomy/redwood"
	"github.com/brynbellomy/redwood/ctx"
)

type app struct {
	ctx.Context
}

var configFilepath = flag.String("config", "", "path to config file")

func main() {
	flag.Parse()
	flag.Set("logtostderr", "true")
	flag.Set("v", "2")

	config, err := rw.ReadConfigAtPath(*configFilepath)
	if err != nil {
		panic(err)
	}

	err = ensureDataDirs(config)
	if err != nil {
		panic(err)
	}

	signingKeypair, err := rw.SigningKeypairFromHDMnemonic(config.HDMnemonicPhrase, rw.DefaultHDDerivationPath)
	if err != nil {
		panic(err)
	}

	encryptingKeypair, err := rw.GenerateEncryptingKeypair()
	if err != nil {
		panic(err)
	}

	txStore := rw.NewBadgerTxStore(config.TxDBRoot(), signingKeypair.Address())
	refStore := rw.NewRefStore(config.RefDataRoot())
	peerStore := rw.NewPeerStore(signingKeypair.Address())
	metacontroller := rw.NewMetacontroller(signingKeypair.Address(), config.StateDBRoot(), txStore, refStore)

	libp2pTransport, err := rw.NewLibp2pTransport(signingKeypair.Address(), config.P2PListenPort, metacontroller, refStore, peerStore)
	if err != nil {
		panic(err)
	}

	tlsCertFilename := filepath.Join(config.DataRoot, "server.crt")
	tlsKeyFilename := filepath.Join(config.DataRoot, "server.key")

	var cookieSecret [32]byte
	copy(cookieSecret[:], []byte(config.HTTPCookieSecret))

	httpTransport, err := rw.NewHTTPTransport(
		signingKeypair.Address(),
		config.HTTPListenHost,
		config.DefaultStateURI,
		metacontroller,
		refStore,
		peerStore,
		signingKeypair,
		cookieSecret,
		tlsCertFilename,
		tlsKeyFilename,
	)
	if err != nil {
		panic(err)
	}

	transports := []rw.Transport{libp2pTransport, httpTransport}

	host, err := rw.NewHost(signingKeypair, encryptingKeypair, transports, metacontroller, refStore, peerStore)
	if err != nil {
		panic(err)
	}

	err = host.Start()
	if err != nil {
		panic(err)
	}

	app := app{}
	app.CtxAddChild(host.Ctx(), nil)
	app.CtxStart(
		func() error { return nil },
		nil,
		nil,
		nil,
	)

	//// Connect to bootstrap peers
	//libp2pTransport := host.Transport("libp2p").(interface{ Libp2pPeerID() string })
	//host2.AddPeer(host2.Ctx(), "libp2p", rw.NewStringSet([]string{"/ip4/0.0.0.0/tcp/21231/p2p/" + libp2pTransport.Libp2pPeerID()}))
	////err = host2.AddPeer(host2.Ctx(), "http", "https://localhost:21232")
	////if err != nil {
	////    panic(err)
	////}
	////err = host1.AddPeer(host1.Ctx(), "http", "https://localhost:21242")
	////if err != nil {
	////    panic(err)
	////}
	//
	//time.Sleep(2 * time.Second)
	//
	//// Both consumers subscribe to the URL
	//ctx, _ := context.WithTimeout(context.Background(), 120*time.Second)
	//go func() {
	//    anySucceeded, _ := host2.Subscribe(ctx, "localhost:21231/gitdemo")
	//    if !anySucceeded {
	//        panic("host2 could not subscribe")
	//    }
	//    anySucceeded, _ = host2.Subscribe(ctx, "localhost:21231/git")
	//    if !anySucceeded {
	//        panic("host2 could not subscribe")
	//    }
	//    anySucceeded, _ = host2.Subscribe(ctx, "localhost:21231/git-reflog")
	//    if !anySucceeded {
	//        panic("host2 could not subscribe")
	//    }
	//}()

	app.AttachInterruptHandler()
	app.CtxWait()
}

func ensureDataDirs(config *rw.Config) error {
	err := os.MkdirAll(config.RefDataRoot(), 0700)
	if err != nil {
		return err
	}

	err = os.MkdirAll(config.TxDBRoot(), 0700)
	if err != nil {
		return err
	}

	err = os.MkdirAll(config.StateDBRoot(), 0700)
	if err != nil {
		return err
	}
	return nil
}
