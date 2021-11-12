import querystring from 'querystring'
import url from 'url'
import WebSocket from 'isomorphic-ws'
import {
    Transport,
    Identity,
    PeersMap,
    PeersCallback,
    Tx,
    SubscribeParams,
    UnsubscribeFunc,
    NewStateMsg,
    NewStateCallbackWithError,
    GetParams
} from './types'

let theFetch: typeof fetch = typeof window !== 'undefined'
                                ? fetch
                                : require('node-fetch')

interface SubscribeHeaders {
    'State-URI': string
    Accept:    string
    Subscribe: SubscribeType
    'From-Tx'?: string
}

type SubscribeType = 'states' | 'transactions' | 'states,transactions' | 'transactions,states'

export default function (opts: { httpHost: string, onFoundPeers?: PeersCallback }) {
    const { httpHost, onFoundPeers } = opts

    let knownPeers: PeersMap = {}
    pollForPeers()

    let alreadyRespondedTo: { [txID: string]: boolean } = {}
    let websocketConn: WebSocket | undefined
    let websocketConnected = false
    let websocketPendingSubscribeOpts: any = []

    let unsubscribes: UnsubscribeFunc[] = []

    async function close() {
        for (let unsubscribe of unsubscribes) {
            unsubscribe()
        }
    }

    let ucan: string | undefined
    function setUcan(newUcan: string) {
        ucan = newUcan
    }

    async function subscribe(opts: SubscribeParams, onOpen: () => void) {
        let { stateURI, keypath, fromTxID, states, txs, callback } = opts
        try {
            let subscriptionType: SubscribeType
            if (states && txs) {
                subscriptionType = 'states,transactions'
            } else if (states) {
                subscriptionType = 'states'
            } else if (txs) {
                subscriptionType = 'transactions'
            } else {
                throw new Error('must provide either `txs: true`, `states: true`, or both')
            }

            let unsubscribe: UnsubscribeFunc

            if (opts.useWebsocket) {
                if (!websocketConn) {
                    let url = new URL(httpHost)
                    url.searchParams.set('state_uri', stateURI)
                    url.searchParams.set('keypath', keypath || '/')
                    url.searchParams.set('subscription_type', subscriptionType)
                    if (fromTxID) {
                        url.searchParams.set('from_tx', fromTxID)
                    }
                    if (url.protocol === 'https:') {
                        url.protocol = 'wss'
                    } else if (url.protocol === 'http:') {
                        url.protocol = 'ws'
                    } else {
                        throw new Error('bad http host: ' + httpHost)
                    }
                    url.pathname = '/ws'

                    if (!!ucan) {
                        url.searchParams.set('ucan', `Bearer ${ucan}`)
                    }
                    websocketConn = new WebSocket(url.toString())
                    websocketConn.onopen = function (evt: any) {
                        websocketConnected = true
                        for (let pendingSubscribeOpts of websocketPendingSubscribeOpts) {
                            if (!websocketConn) {
                                continue
                            }
                            websocketConn.send(JSON.stringify({
                                op: 'subscribe',
                                params: pendingSubscribeOpts,
                            }))
                        }
                    }
                    websocketConn.onclose = function (evt: any) {
                        websocketConnected = false
                    }
                    websocketConn.onmessage = function (evt: any) {
                        let messages = (evt.data as string).split('\n').filter(x => x.trim().length > 0)
                        for (let msg of messages) {
                            if (!websocketConn) {
                                continue
                            }
                            if (msg === 'ping') {
                                websocketConn.send('pong')
                                continue
                            }

                            try {
                                let { stateURI, tx, state, leaves } = JSON.parse(msg)
                                callback(null, { stateURI, tx, state, leaves })
                            } catch (err) {
                                callback(err, undefined as any)
                            }
                        }
                    }

                    unsubscribes.push(() => {
                        websocketConn?.close()
                        websocketConn = undefined
                    })

                } else {
                    let subscribeOpts = { stateURI, keypath, subscriptionType, fromTxID }
                    if (websocketConnected) {
                        websocketConn.send(JSON.stringify({
                            op: 'subscribe',
                            params: subscribeOpts,
                        }))
                    } else {
                        websocketPendingSubscribeOpts.push(subscribeOpts)
                    }
                }
                unsubscribe = () => {
                    // if (websocketConn) {
                    //     websocketConn.close()
                    //     websocketConn = undefined
                    // }
                }

            } else {
                const headers: SubscribeHeaders = {
                    'State-URI': stateURI,
                    'Accept':    'application/json',
                    'Subscribe': subscriptionType,
                }
                if (fromTxID) {
                    headers['From-Tx'] = fromTxID
                }

                const resp = await wrappedFetch(keypath || '/', {
                    method: 'GET',
                    headers,
                })
                if (!resp.ok || !resp.body) {
                    callback('http transport: fetch failed', undefined as any)
                    return
                }
                unsubscribe = readSubscription(stateURI, resp.body.getReader(), (err, update) => {
                    if (err) {
                        callback(err, undefined as any)
                        return
                    }
                    let { stateURI, tx, state, leaves } = update
                    if (tx) {
                        ack(tx.id)
                        if (!alreadyRespondedTo[tx.id]) {
                            alreadyRespondedTo[tx.id] = true
                            callback(err, { stateURI, tx, state, leaves })
                        }
                    } else {
                        callback(err, { stateURI, tx, state, leaves })
                    }
                })
            }
            unsubscribes.push(unsubscribe)

            onOpen()

            return unsubscribe

        } catch (err) {
            throw err
            callback('http transport: ' + err, undefined as any)
            return () => {}
        }
    }

    function readSubscription(stateURI: string, reader: ReadableStreamDefaultReader<Uint8Array>, callback: NewStateCallbackWithError) {
        let shouldStop = false
        function unsubscribe() {
            shouldStop = true
            reader.cancel()
        }

        setTimeout(async () => {
            try {
                const decoder = new TextDecoder('utf-8')
                let buffer = ''

                async function read() {
                    const x = await reader.read()
                    if (x.done) {
                        return
                    }

                    const newData = decoder.decode(x.value)
                    buffer += newData
                    let idx
                    while ((idx = buffer.indexOf('\n')) > -1) {
                        if (shouldStop) {
                            return
                        }
                        const line = buffer.substring(0, idx).trim()
                        if (line.length > 0) {
                            const payloadStr = line.substring(5).trim() // remove "data:" prefix
                            let payload
                            try {
                                payload = JSON.parse(payloadStr)
                            } catch (err) {
                                console.error('Error parsing JSON:', payloadStr)
                                callback('http transport: ' + err, undefined as any)
                                return
                            }
                            callback(null, payload)

                        }
                        buffer = buffer.substring(idx+1)
                    }
                    if (shouldStop) {
                        return
                    }
                    read()
                }
                read()

            } catch (err) {
                callback('http transport: ' + err, undefined as any)
                return
            }
        }, 0)
        return unsubscribe
    }

    async function get({ stateURI, keypath, raw }: GetParams) {
        let url = keypath || '/'
        if (url.length > 0 && url[0] !== '/') {
            url = '/' + url
        }
        if (raw) {
            url = url + '?raw=1'
        }
        return (await (await wrappedFetch(url, {
            headers: {
                'Accept': 'application/json',
                'State-URI': stateURI,
            },
        })).json()) as any
    }

    // @@TODO: private tx functionality
    async function put(tx: Tx) {
        let body: FormData | string
        if (tx.attachment) {
            let fd: FormData
            if (typeof window !== 'undefined') {
                fd = new FormData()
            } else {
                let FormData = require('form-data')
                fd = new FormData()
            }
            fd.append('attachment', tx.attachment)
            fd.append('patches', tx.patches.join('\n'))
            body = fd

        } else {
            body = tx.patches.join('\n')
        }

        await wrappedFetch('/', {
            method: 'PUT',
            body: body,
            headers: {
                'State-URI': tx.stateURI,
                'Version': tx.id,
                'Parents': (tx.parents || []).join(','),
                'Signature': tx.sig,
                'Patch-Type': 'braid',
            },
        })
    }

    async function ack(txID: string) {
        await wrappedFetch('/', {
            method: 'ACK',
            body: txID,
        })
    }

    async function storeBlob(file: string | Blob) {
        let formData
        if (typeof window !== 'undefined') {
            formData = new FormData()
            formData.append('blob', file)
        } else {
            let FormData = require('form-data')
            formData = new FormData()
            formData.append('blob', file)
        }

        const resp = await wrappedFetch(`/`, {
            method: 'POST',
            headers: {
                'Blob': 'true',
            },
            body: formData,
        })

        return (await resp.json())
    }

    async function authorize(identity: Identity) {
        const resp = await wrappedFetch(`/`, {
            method: 'AUTHORIZE',
        })

        const challengeHex = await resp.text()
        const challenge = Buffer.from(challengeHex, 'hex')
        const sigHex = identity.signBytes(challenge)

        const resp2 = await wrappedFetch(`/`, {
            method: 'AUTHORIZE',
            headers: {
                'Response': sigHex,
            },
        })
    }

    let cookies: { [cookie: string]: string } = {}

    async function wrappedFetch(path: string, options: any) {
        if (typeof window === 'undefined') {
            // We have to manually parse and set cookies because isomorphic-fetch doesn't do it for us
            let cookieStr = Object.keys(cookies).map(cookieName => `${cookieName}=${cookies[cookieName]}`).join(';')
            options.headers = {
                ...makeRequestHeaders(),
                ...options.headers,
                Cookie: cookieStr,
            }

        } else {
            options.headers = {
                ...makeRequestHeaders(),
                ...options.headers,
            }
        }
        options.credentials = 'include'

        path = path || ''
        if (path[0] !== '/') {
            path = '/' + (path || '')
        }

        let url = !httpHost ? path : httpHost + path

        const resp = await theFetch(url, options)
        if (!resp.ok) {
            let text = await resp.text()
            throw { statusCode: resp.status, error: text }
        }

        if (typeof window === 'undefined') {
            // Manual cookie parsing
            let rawHeaders: { [k: string]: string[] } = (resp.headers as any).raw()
            for (let str of (rawHeaders['set-cookie'] || [])) {
                let keyVal = str.substr(0, str.indexOf(';')).split('=')
                cookies[keyVal[0]] = keyVal[1]
            }
        }

        // Receive list of peers from the Alt-Svc header
        const altSvcHeader = resp.headers.get('Alt-Svc')
        if (altSvcHeader) {
            const peers: PeersMap = {}
            const peerHeaders = altSvcHeader.split(',').map(x => x.trim())
            for (let peer of peerHeaders) {
                const x = peer.match(/^\s*(\w+)="([^"]+)"/)
                if (!x) { continue }
                const tptName = x[1]
                const reachableAt = x[2]
                peers[tptName] = peers[tptName] || {}
                peers[tptName][reachableAt] = true
            }
            if (onFoundPeers) {
                onFoundPeers(peers)
            }
        }
        return resp
    }

    function pollForPeers() {
        setInterval(async () => {
            try {
                await wrappedFetch(`/`, { method: 'HEAD' })
            } catch(err) {
                console.error('pollForPeers error ~>', err)
            }

        }, 5000)
    }

    function makeRequestHeaders() {
        const headers: { [header: string]: string } = {}
        if (ucan) {
            headers['Authorization'] = `Bearer ${ucan}`
        }

        const altSvc = []
        for (let tptName of Object.keys(knownPeers)) {
            for (let reachableAt of Object.keys(knownPeers[tptName])) {
                altSvc.push(`${tptName}="${reachableAt}"`)
            }
        }
        if (altSvc.length > 0) {
            headers['Alt-Svc'] = altSvc.join(', ')
        }
        return headers
    }

    function foundPeers(peers: PeersMap) {
        knownPeers = peers
    }

    return {
        transportName:   () => 'http',
        altSvcAddresses: () => [],
        setUcan,
        subscribe,
        get,
        put,
        ack,
        storeBlob,
        authorize,
        foundPeers,
        close,
    } as Transport
}
