import * as identity from './identity'
import * as sync9 from './resolver.sync9.browser'
import * as dumb from './resolver.dumb.browser'
import * as utils from './utils'
import httpTransport from './transport.http'
// import * as webrtcTransport from './transport.webrtc'
import rpcTransport from './transport.rpc'

export default {
    createPeer,

    // submodules
    identity,
    sync9,
    dumb,
    utils,
}

function createPeer(opts) {
    const { httpHost, identity, webrtc, onFoundPeersCallback, rpcEndpoint } = opts

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
        let unsubscribers = (await Promise.all(
            transports.map(tpt => tpt.subscribe({ stateURI, keypath, fromTxID, states, txs, callback }))
        )).filter(unsub => !!unsub)
        return () => {
            for (let unsub of unsubscribers) {
                if (unsub) {
                    unsub()
                }
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
        rpc: rpcTransport(rpcEndpoint || 'http://localhost:8081'),
    }
}

