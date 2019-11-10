

module.exports = function (opts) {
    const { onFoundPeers } = opts

    let myPeerID
    let onTxReceived
    let knownPeers = {}
    const conns = {}
    const me = new Peer()
    me.on('open', async (_myPeerID) => {
        myPeerID = _myPeerID
        onFoundPeers({ webrtc: { [myPeerID]: true } })
    })
    me.on('connection', (conn) => {
        conns[conn.peer] = conn
        initConnCallbacks(conn)
    })

    function initConnCallbacks(conn) {
        conn.on('open', () => {
            console.log('webrtc: connected to ' + conn.peer)
        })
        conn.on('data', (data) => {
            const tx = JSON.parse(data)
            if (onTxReceived) {
                onTxReceived(null, tx)
            }
        })
        conn.on('close', () => {
            delete conns[conn.peer]
        })
    }

    function subscribe(stateURI, keypath, parents, _onTxReceived) {
        onTxReceived = _onTxReceived
        for (let peerID of Object.keys(conns)) {
            conns[peerID].on('data', (data) => {
                const tx = JSON.parse(data)
                onTxReceived(null, tx)
            })
        }
    }

    function connect(remoteID) {
        const conn = me.connect(remoteID, { reliable: true })
        conns[remoteID] = conn
        initConnCallbacks(conn)
    }

    return {
        transportName:   () => 'webrtc',
        altSvcAddresses: () =>  myPeerID ? [myPeerID] : [],
        subscribe,

        foundPeers: (peers) => {
            knownPeers = peers
            Object.keys(peers.webrtc || {})
                .filter(p => p !== myPeerID)
                .filter(p => !conns[p])
                .forEach(p => connect(p))
        },

        put: (tx) => {
            Object.keys(conns).forEach(peerID => {
                try {
                    conns[peerID].send(JSON.stringify(tx))
                } catch (err) {
                    console.error('error sending to peer ~>', err)
                }
            })
        },
    }
}