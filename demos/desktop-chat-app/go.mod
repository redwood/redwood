module github.com/brynbellomy/redwood-webview

go 1.15

replace github.com/brynbellomy/redwood => ../../

require (
	github.com/brynbellomy/klog v0.0.0-20200414031930-87fbf2e555ae
	github.com/brynbellomy/redwood v0.0.0-20210118002547-53857574d4c7
	github.com/dgraph-io/badger/v2 v2.2007.2 // indirect
	github.com/libp2p/go-libp2p v0.13.0 // indirect
	github.com/libp2p/go-libp2p-host v0.1.0 // indirect
	github.com/libp2p/go-libp2p-kad-dht v0.11.1 // indirect
	github.com/libp2p/go-libp2p-metrics v0.1.0 // indirect
	github.com/libp2p/go-libp2p-protocol v0.1.0 // indirect
	github.com/markbates/pkger v0.17.1
	github.com/pkg/errors v0.9.1
	github.com/rs/cors v1.7.0 // indirect
	github.com/urfave/cli v1.22.4
	github.com/webview/webview v0.0.0-20200724072439-e0c01595b361
	google.golang.org/protobuf v1.25.0 // indirect
)
