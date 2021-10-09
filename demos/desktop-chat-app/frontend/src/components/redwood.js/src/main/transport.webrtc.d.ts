export default function _default(opts: any): {
    transportName: () => string;
    altSvcAddresses: () => any[];
    subscribe: ({ stateURI, keypath, fromTxID, states, txs, callback }: {
        stateURI: any;
        keypath: any;
        fromTxID: any;
        states: any;
        txs: any;
        callback: any;
    }) => void;
    foundPeers: (peers: any) => void;
    put: (tx: any) => void;
};
//# sourceMappingURL=transport.webrtc.d.ts.map