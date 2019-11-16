
# 🌲 Redwood

Redwood is a **highly-configurable, distributed, realtime database** that manages a state tree shared among many peers.  Imagine something like a Redux store, but distributed across all users of an application, that offers offline editing and is resilient to poor connectivity.

Redwood is also an **application server**.  Developers can store and update assets (HTML, Javascript, images) directly in the state tree.  For many types of applications, you may not need a separate backend server at all.

Its flexibility allows developers to use a single, simple programming model to create many divergent classes of applications:
- Traditional web applications
- Realtime collaborative document editors
- Peer-to-peer encrypted messaging
- Blockchains

## 🧬 The Braid protocol

Redwood is part of the [Braid project](https://braid.news), and uses the Braid protocol to synchronize updates over regular HTTP.  [Braid is currently working through the IETF's RFC process to have its extensions to HTTP standardized](https://github.com/braid-work/braid-spec)  (see IETF draft spec here: <https://tools.ietf.org/html/draft-toomim-braid-00>)

## ✔️ Redwood's features

- **Javascript client:** Redwood ships with Braid.js, a Javascript client that allows browsers to communicate with one another and with Redwood's Go nodes.
- **Accounts/identity:** Redwood uses ECDSA asymmetric cryptography to assign a decentralized identity (also known as a DID, but in this project usually referred to as an "address") to every peer.
- **Transaction model:** Every update to the database is a transaction, signed by its sender, that can contain one or more patches.  Patches use a simple Javascript-like language to describe updates to the state tree.
- **Private transactions and subtrees:** Users can send private, encrypted transactions to single peers or groups.  The recipients will all share a private subtree that's invisible to the rest of the swarm.
- **Merge resolution:** Redwood expects downtime, loss of connectivity, and simultaneous conflicting edits.  To address these obstacles, it allows the developer to configure the state tree with a variety of merge resolvers.  The main one in use at the moment is called Sync9.  Different keypaths can have different resolvers.  Custom resolvers can be written in:
    - Go
    - Javascript (executed using Chrome's V8 engine)
    - Lua
- **Asset storage:** Assets like HTML and Javascript files can be stored in the state tree as well.  The state tree _is_ your application.  See the included demos for examples.
- **Transports:** Redwood implements several transports, including [libp2p](https://libp2p.io), [Braid-over-HTTP](https://braid.news), and WebRTC.
    - The Go nodes communicate with one another over libp2p or HTTP (configurable)
    - The browser nodes communicate with one another over WebRTC
    - The browser nodes communicate with the Go nodes over HTTP
    - After a set of browser nodes connect with one another, you can kill the Go nodes, and the browsers can still talk to one another.
- **Git integration:** (WIP) Soon, Redwood will be able to manage a Git repository for you.  When you push updates to the code and assets in your application, commits will be created and pushed to traditional Git remotes.  Redwood will also be able to understand incoming Git commits as Redwood transactions.  See [git-resolver/resolver.git.go](https://github.com/brynbellomy/redwood/blob/master/git-resolver/resolver.git.go) for a draft version of this functionality.


## The demos

Each demo comes with a debugging view that allows you to inspect the current state tree, the full list of transactions, and the swarm's current network topology.

### Demo #1: chat room

You can run the chat demo with the following commands (note: Go 1.13+ is required).

```sh
$ cd scenarios/talk-channel
$ go run talk-channel.go
```

This will spin up two nodes in the same process.  Once they're up, open browser tabs to:
- <https://localhost:21232/shrugisland/talk0/index>
- <https://localhost:21242/shrugisland/talk0/index>

**(Note: due to the self-signed TLS certificates, your browser will complain that you're "not safe")**

You now have four nodes (two in Go, two in the browser) that can talk with one another.

### Demo #2: collaborative text editor

You can run the text editor demo with the following commands (note: Go 1.13+ is required).

```sh
$ cd scenarios/text-editor
$ go run main.go
```

This will spin up two nodes in the same process.  Once they're up, open browser tabs to:
- <https://localhost:21232/editor/index>
- <https://localhost:21242/editor/index>

**(Note: due to the self-signed TLS certificates, your browser will complain that you're "not safe")**

As with the chat room demo, you now have four nodes (two in Go, two in the browser) that can talk with one another.



