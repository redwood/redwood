# Redwood bootstrap node

The bootstrap node serves a single function: to run behind some publicly-accessible network address and assist full Redwood nodes in finding one another if they are not publicly-accessible.

Peerstore and DHT state are persisted to disk.

Config is managed via a JSON file of the format:

```json
{
    "port": 21231,
    "p2pKey": "<base64-encoded libp2p private key protobuf>",
    "datastore": {
        "path": "./data",
        "encryption": {
            "key": "<hex-encoded AES-256 key>",
            "rotationInterval": 86400000000000
        }
    },
    "bootstrapPeers": [ "<libp2p multiaddress>" ],
    "dnsOverHTTPSURL": "<url to a DNS-over-HTTPS endpoint>"
}
```

### Generating a config

You might want to auto-generate an initial config, particularly if you don't already have a libp2p  private key and/or a libp2p datastore key. To do so, simply run:

```sh
bootstrapnode genconfig --config ./config.json
```

### Running the node

```sh
bootstrapnode start --config ./config.json
```

The bootstrap node will report its libp2p public key, which you will need to distribute to any Redwood users interested in using this node to find peers. Those users will need to add an entry to the `StaticRelays` section of their `.redwoodrc` configuration files using the libp2p multiaddress format, e.g.:

```
/dns4/bootstrap.chat.org/tcp/21231/p2p/12D3KooWCt4YfyFNL1W4P58oYwznJGPA5G7JtFch79PEpM1UMB92
```

