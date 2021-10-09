var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
import Peer from 'peerjs';
export default function (opts) {
    let { onFoundPeers, onLostPeers } = opts;
    onFoundPeers = onFoundPeers || function () { };
    onLostPeers = onLostPeers || function () { };
    let webrtcPeerID;
    let onTxReceived;
    let knownPeers = {};
    const conns = {};
    const me = new Peer();
    me.on('open', (_webrtcPeerID) => __awaiter(this, void 0, void 0, function* () {
        webrtcPeerID = _webrtcPeerID;
        console.log('webrtc: i am', webrtcPeerID);
        onFoundPeers({ webrtc: { [webrtcPeerID]: true } });
    }));
    me.on('connection', (conn) => {
        conns[conn.peer] = conn;
        initConnCallbacks(conn);
    });
    function initConnCallbacks(conn) {
        conn.on('open', () => {
            console.log('webrtc: connected to ' + conn.peer);
            onFoundPeers({ webrtc: { [conn.peer]: true } });
        });
        conn.on('data', (data) => {
            const tx = JSON.parse(data);
            if (onTxReceived) {
                onTxReceived(null, { tx });
            }
        });
        conn.on('close', () => {
            onLostPeers({ webrtc: { [webrtcPeerID]: true } });
            delete conns[conn.peer];
        });
    }
    function subscribe({ stateURI, keypath, fromTxID, states, txs, callback }) {
        onTxReceived = _onTxReceived;
        for (let peerID of Object.keys(conns)) {
            conns[peerID].on('data', (data) => {
                const tx = JSON.parse(data);
                console.log('webrtc: received from peer', peerID, '~>', tx);
                onTxReceived(null, { tx });
            });
        }
    }
    function connect(remoteID) {
        const conn = me.connect(remoteID, { reliable: true });
        conns[remoteID] = conn;
        initConnCallbacks(conn);
    }
    return {
        transportName: () => 'webrtc',
        altSvcAddresses: () => webrtcPeerID ? [webrtcPeerID] : [],
        subscribe,
        foundPeers: (peers) => {
            knownPeers = peers;
            for (let reachableAt of Object.keys(peers.webrtc || {})) {
                if (reachableAt !== webrtcPeerID && !conns[reachableAt]) {
                    connect(reachableAt);
                }
            }
        },
        put: (tx) => {
            Object.keys(conns).forEach(peerID => {
                try {
                    console.log('webrtc: sending to peer', peerID, '~>', tx);
                    conns[peerID].send(JSON.stringify(tx));
                }
                catch (err) {
                    console.error('webrtc: error sending to peer ~>', err);
                }
            });
        },
    };
}
