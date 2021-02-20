
# 🌲 Redwood

![Build status](https://github.com/brynbellomy/redwood/actions/workflows/go_tests.yml/badge.svg?branch=develop)

Redwood is a **highly-configurable, distributed, realtime database** that manages a state tree shared among many peers.  Imagine something like a Redux store, but distributed across all users of an application, that offers offline editing and is resilient to poor connectivity.

Redwood is also an **application server**.  Developers can store and update assets (HTML, Javascript, images) directly in the state tree.  For many types of applications, you may not need a separate backend server at all.

Its flexibility allows developers to use a single, simple programming model to create many divergent classes of applications:
- Traditional web applications
- Realtime collaborative document editors
- Peer-to-peer encrypted messaging
- Blockchains
- Git-style version control systems

## 🧬 The Braid protocol

Redwood is part of the [Braid project](https://braid.org), and uses the Braid protocol to synchronize updates over regular HTTP.  [Braid is currently working through the IETF's RFC process to have its extensions to HTTP standardized](https://github.com/braid-org/braid-spec)  (see IETF draft spec here: <https://datatracker.ietf.org/doc/html/draft-toomim-httpbis-braid-http>)

## 🧮 Demos

Demos can be found in the [demos](https://github.com/brynbellomy/redwood/tree/master/demos) folder.  Each one has its own README with instructions.

Each demo comes with a debugging view that allows you to inspect the current state tree, the full list of transactions, and the swarm's current network topology.

Demos:
- [Realtime collaborative document editor](https://github.com/brynbellomy/redwood/tree/master/demos/text-editor)
- [Chat room](https://github.com/brynbellomy/redwood/tree/master/demos/chat)
- [Git integration](https://github.com/brynbellomy/redwood/tree/master/demos/git-integration)
- [Desktop chat app](https://github.com/brynbellomy/redwood/tree/master/demos/desktop-chat-app) (WIP, early development)
- [Video chat app](https://github.com/brynbellomy/redwood/tree/master/demos/video-chat-app) (WIP, early development)

## ✔️ Redwood's features

⚠️  **Redwood is still pre-alpha software** ⚠️ , but it already includes a number of compelling features.  The quality of these features are being improved continually, and new features are added on a regular basis.

Keep an eye on our [Github project board](https://github.com/brynbellomy/redwood/projects/1) if you're interested in our roadmap.

### Features

- **Accounts/identity and access control:** Redwood uses ECDSA asymmetric cryptography to assign a decentralized identity (also known as a DID, but in this project usually referred to as an "address") to every peer.  Access control is easy to configure, and can be applied to any subtree.
    - Yubikey support (and support for other types of hardware keys) is in the works.
- **Transaction model:** Every update to the database is a transaction, signed by its sender, that can contain one or more patches.  Patches use a simple Javascript-like language to describe updates to the state tree.
- **Private transactions and subtrees:** Users can send private, encrypted transactions to single peers or groups.  The recipients will all share a private subtree that's invisible to the rest of the swarm.
- **Merge resolution:** Redwood expects downtime, loss of connectivity, and simultaneous conflicting edits.  To address these obstacles, it allows the developer to configure the state tree with a variety of merge resolvers.  The main one in use at the moment is called Sync9.  Different keypaths can have different resolvers.  Custom resolvers can be written in:
    - Go
    - Javascript (executed using Chrome's V8 engine)
    - Lua
    - WASM (forthcoming)
- **Asset storage:** Assets like HTML and Javascript files can be stored in the state tree as well.  The state tree _is_ your application.  See the included demos for examples.
- **Transports:** Redwood implements several transports, including [libp2p](https://libp2p.io), [Braid-over-HTTP](https://braid.org), and [WebRTC](https://webrtc.org/).
    - The Go nodes communicate with one another over libp2p or HTTP (configurable)
    - The browser nodes communicate with one another over WebRTC
    - The browser nodes communicate with the Go nodes over HTTP
    - After a set of browser nodes connect with one another, you can kill the Go nodes, and the browsers can still talk to one another.
- **Clients:**
    - **Braid.js:** Redwood ships with Braid.js, a Javascript client that allows browsers to communicate with one another and with Redwood's Go nodes.
    - **Go HTTP client:** Redwood includes a Go implementation of a Braid HTTP client.
    - **Go protobuf client (forthcoming):** Redwood will include a Go protobuf client in the very near future.
- **Git integration:**
    - Redwood can act as a Git server.  With the included Git remote helper plugin, you can do things like `git clone redwood://mysite.com/git`, and also push and pull, without actually setting up a Git server of any kind.
    - When you push updates to the code and assets in your application, they will be deployed instantly with zero downtime.  No more blue-green deploys.  See [git-remote-helper/main.go](https://github.com/brynbellomy/redwood/blob/master/git-remote-helper/main.go) for the remote helper itself, and [demos/git-integration/main.go](https://github.com/brynbellomy/redwood/blob/master/demos/git-integration/main.go), the fully-featured demo, for more information.  Instructions for running the Git demo are provided below.


## 🔒 Security model

(This section is in draft, although the information it contains is up to date)

Redwood has a goal of providing a multilayered, robust security model.  It currently implements the following mechanisms to achieve this goal:

1. Users are identified by a public/private keypair (actually using Ethereum's implementation at the moment).  All transactions must be signed by the sender, allowing recipients to verify the sender identities.

2. You can place "transaction validators" at any node in your state trees, and any transaction affecting the subtree under the validator will be checked by that validator.  Currently, there's a "permissions" validator included that gives a simple way to control writes based on the Ethereum keypair I mentioned above.  This part isn't very well fleshed out yet, but it provides what seems to be a solid model to iterate on.

3. You can also write custom transaction validators in Go/Lua/Javascript, which should make it trivial to implement just about any access control model you desire.

4. You can also create a "private" tree by explicitly specifying the set of users who are allowed to read from and write to that tree.  The default Redwood node implementation does automatic peer discovery and keeps a list of peers whose identities/addresses have been verified.  When it receives a transaction for a private tree, it only gossips that transaction to the tree's members (as opposed to its behavior with public trees, which is to gossip transactions to any peer who subscribes to that tree).

5. We also want secure persistent storage for transactions so that we can get high availability and redundancy without compromising the security model.  To facilitate this, we allow you to configure the node to talk to what we're calling a "remote blind store" (essentially a key-value DB running on a separate piece of hardware, possibly off-site).  The Redwood node encrypts transactions with a key the remote store doesn't possess and sends it over gRPC to the blind store.

6. Some applications will have a high volume of transaction data, and will want to be able to prune/merge/rebase that data to save space.  To prevent bad actors from issuing malicious prune requests, the remote blind stores are going to implement their own p2p protocol involving threshold signatures.  Prune requests will only be honored if a quorum of these blind stores sign off on them.




