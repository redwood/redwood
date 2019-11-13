require('@babel/polyfill')

const identity = require('./identity')
const sync9 = require('./sync9-src')
const utils = require('./utils')
const httpTransport = require('./braid.transport.http')
const webrtcTransport = require('./braid.transport.webrtc')

var Braid = {
    createPeer,

    // submodules
    identity,
    sync9,
    utils,
}

if (window) {
    window.Braid = Braid
}
if (module && module.exports) {
    module.exports = Braid
}

function createPeer(opts) {
    const { httpHost, identity, webrtc, onFoundPeersCallback } = opts

    const transports = [ httpTransport({ onFoundPeers, httpHost }) ]
    if (webrtc === true) {
        transports.push(webrtcTransport({ onFoundPeers }))
    }

    const knownPeers = {}
    function onFoundPeers(peers) {
        for (let transportName of Object.keys(peers)) {
            knownPeers[transportName] = {
                ...knownPeers[transportName],
                ...peers[transportName],
            }
        }

        transports.forEach(tpt => tpt.foundPeers(knownPeers))

        if (onFoundPeersCallback) {
            onFoundPeersCallback(knownPeers)
        }
    }

    async function subscribe(stateURI, keypath, parents, onTxReceived) {
        for (let tpt of transports) {
            if (tpt.subscribe) {
                tpt.subscribe(stateURI, keypath, parents, onTxReceived)
            }
        }
    }

    async function get(stateURI, keypath) {
        for (let tpt of transports) {
            if (tpt.get) {
                return tpt.get(stateURI, keypath)
            }
        }
    }

    async function put(tx) {
        tx.from = identity.address
        tx.sig = identity.signTx(tx)

        for (let tpt of transports) {
            if (tpt.put) {
                try {
                    await tpt.put(tx)
                } catch (err) {
                    console.error('error PUTting to peer ~>', err)
                }
            }
        }
    }

    async function storeRef(file) {
        let hash
        for (let tpt of transports) {
            if (tpt.storeRef) {
                hash = await tpt.storeRef(file)
            }
        }
        // @@TODO: check if different?
        return hash
    }

    async function authorize() {
        for (let tpt of transports) {
            if (tpt.authorize) {
                await tpt.authorize(identity)
            }
        }
    }

    return {
        get,
        subscribe,
        put,
        storeRef,
        authorize,
    }
}

