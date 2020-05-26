require('es6-promise').polyfill()
require('isomorphic-fetch')

module.exports = function (opts) {
    const { httpHost, onFoundPeers, peerID } = opts

    let knownPeers = {}
    pollForPeers()

    async function subscribe(stateURI, keypath, parents, onTxReceived) {
        try {
            const headers = {
                'State-URI': stateURI,
                'Accept':    'application/json',
                'Subscribe': 'keep-alive',
            }
            if (parents && parents.length > 0) {
                headers['Parents'] = parents.join(',')
            }

            const resp = await wrappedFetch(keypath, {
                method: 'GET',
                headers,
            })
            if (!resp.ok) {
                onTxReceived('http transport: fetch failed')
                return
            }
            const reader = resp.body.getReader()
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
                    const line = buffer.substring(0, idx).trim()
                    if (line.length > 0) {
                        const payloadStr = line.substring(5).trim() // remove "data:" prefix
                        let payload
                        try {
                            payload = JSON.parse(payloadStr)
                        } catch (err) {
                            console.error('Error parsing JSON:', payloadStr)
                            onTxReceived('http transport: ' + err)
                            return
                        }
                        onTxReceived(null, payload)
                    }
                    buffer = buffer.substring(idx+1)
                }
                read()
            }
            read()

        } catch (err) {
            onTxReceived('http transport: ' + err)
            return
        }
    }

    async function get({ stateURI, keypath, raw }) {
        if (keypath.length > 0 && keypath[0] !== '/') {
            keypath = '/' + keypath
        }
        if (raw) {
            keypath = keypath + '?raw=1'
        }
        return (await (await wrappedFetch(keypath, {
            headers: {
                'Accept': 'application/json',
                'State-URI': stateURI,
            },
        })).json())
    }

    function put(tx) {
        return wrappedFetch('/', {
            method: 'PUT',
            body: tx.patches.join('\n'),
            headers: {
                'State-URI': tx.stateURI,
                'Version': tx.id,
                'Parents': (tx.parents || []).join(','),
                'Signature': tx.sig,
                'Patch-Type': 'braid',
            },
        })
    }

    async function storeRef(file) {
        let formData
        if (typeof window !== 'undefined') {
            formData = new FormData()
            formData.append('ref', file)
        } else {
            let FormData = require('form-data')
            formData = new FormData()
            formData.append('ref', file)
        }

        const resp = await wrappedFetch(`/`, {
            method: 'PUT',
            headers: {
                'Ref': 'true',
            },
            body: formData,
        })

        return (await resp.json())
    }

    async function authorize(identity) {
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

    async function wrappedFetch(path, options) {
        options.headers = {
            ...makeRequestHeaders(),
            ...options.headers,
        }

        const resp = await fetch(httpHost + path, options)
        const altSvcHeader = resp.headers.get('Alt-Svc')
        if (altSvcHeader) {
            const peers = {}
            const peerHeaders = altSvcHeader.split(',').map(x => x.trim())
            for (let peer of peerHeaders) {
                const x = peer.match(/^\s*(\w+)="([^"]+)"/)
                const tptName = x[1]
                const reachableAt = x[2]
                peers[tptName] = peers[tptName] || {}
                peers[tptName][reachableAt] = true
            }
            onFoundPeers(peers)
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
        const headers = {}
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

    function foundPeers(peers) {
        knownPeers = peers
    }

    return {
        transportName:   () => 'http',
        altSvcAddresses: () => [],
        subscribe,
        get,
        put,
        storeRef,
        authorize,
        foundPeers,
    }
}
