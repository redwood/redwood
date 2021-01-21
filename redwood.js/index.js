import * as identity from './identity'
import * as sync9 from './sync9-src'
import * as dumb from './dumb-src'
import * as utils from './utils'
import httpTransport from './redwood.transport.http'
// import * as webrtcTransport from './redwood.transport.webrtc'

export default {
    createPeer,

    // submodules
    identity,
    sync9,
    dumb,
    utils,
}

function createPeer(opts) {
    const { httpHost, identity, webrtc, onFoundPeersCallback } = opts

    const transports = [ httpTransport({ onFoundPeers, httpHost, peerID: identity.peerID }) ]
    // if (webrtc === true) {
    //     transports.push(webrtcTransport({ onFoundPeers, peerID: identity.peerID }))
    // }

    let knownPeers = {}
    function onFoundPeers(peers) {
        knownPeers = utils.deepmerge(knownPeers, peers)

        transports.forEach(tpt => tpt.foundPeers(knownPeers))

        if (onFoundPeersCallback) {
            onFoundPeersCallback(knownPeers)
        }
    }

    async function peers() {
        return knownPeers
    }

    async function authorize() {
        for (let tpt of transports) {
            if (tpt.authorize) {
                await tpt.authorize(identity)
            }
        }
    }

    async function subscribe({ stateURI, keypath, fromTxID, states, txs, callback }) {
        for (let tpt of transports) {
            if (tpt.subscribe) {
                tpt.subscribe({ stateURI, keypath, fromTxID, states, txs, callback })
            }
        }
    }

    async function get({ stateURI, keypath, raw }) {
        for (let tpt of transports) {
            if (tpt.get) {
                return tpt.get({ stateURI, keypath, raw })
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

    return {
        identity,
        get,
        subscribe,
        put,
        storeRef,
        authorize,
        peers,
    }
}

