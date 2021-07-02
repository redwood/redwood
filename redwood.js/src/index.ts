import * as identity from './identity'
import * as sync9 from './resolver.sync9.browser'
import * as dumb from './resolver.dumb.browser'
import * as utils from './utils'
import httpTransport from './transport.http'
// import * as webrtcTransport from './transport.webrtc'
import rpcTransport from './transport.rpc'
import {
    Transport,
    Identity,
    PeersMap,
    PeersCallback,
    SubscribeParams,
    GetParams,
    Tx,
    StoreBlobResponse,
} from './types'

export * from './types'

export default {
    createPeer,

    // submodules
    identity,
    sync9,
    dumb,
    utils,
}

interface CreatePeerOptions {
    httpHost: string
    identity?: Identity
    webrtc?: boolean
    onFoundPeersCallback?: PeersCallback
    rpcEndpoint?: string
}

function createPeer(opts: CreatePeerOptions) {
    const { httpHost, identity, webrtc, onFoundPeersCallback, rpcEndpoint } = opts

    const http = httpTransport({ onFoundPeers, httpHost })
    const transports: Transport[] = [ http ]
    // if (webrtc === true) {
    //     transports.push(webrtcTransport({ onFoundPeers, peerID: identity.peerID }))
    // }

    let knownPeers: PeersMap = {}
    function onFoundPeers(peers: PeersMap) {
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
        if (!identity) {
            throw new Error('cannot .authorize() without an identity')
        }
        for (let tpt of transports) {
            if (tpt.authorize) {
                await tpt.authorize(identity)
            }
        }
    }

    async function subscribe(params: SubscribeParams) {
        let unsubscribers = (await Promise.all(
            transports.map(tpt => tpt.subscribe(params))
        )).filter(unsub => !!unsub)
        return () => {
            for (let unsub of unsubscribers) {
                if (unsub) {
                    unsub()
                }
            }
        }
    }

    async function get({ stateURI, keypath, raw }: GetParams) {
        for (let tpt of transports) {
            if (tpt.get) {
                return tpt.get({ stateURI, keypath, raw })
            }
        }
    }

    async function put(tx: Tx) {
        if (!identity) {
            throw new Error('cannot .put() without an identity')
        }
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

    async function storeBlob(file: string | Blob) {
        for (let tpt of transports) {
            if (tpt.storeBlob) {
                return tpt.storeBlob(file)
            }
        }
        throw new Error('no transports support storeBlob')
    }

    async function close() {
        for (let tpt of transports) {
            await tpt.close()
        }
    }

    return {
        identity,
        get,
        subscribe,
        put,
        storeBlob,
        authorize,
        peers,
        rpc: rpcEndpoint ? rpcTransport({ endpoint: rpcEndpoint }) : undefined,
        close,
    }
}

