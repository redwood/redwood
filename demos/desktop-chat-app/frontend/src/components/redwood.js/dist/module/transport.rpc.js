var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
let theFetch = typeof window !== "undefined" ? fetch : require("node-fetch");
function rpcFetch(endpoint, method, params) {
    return __awaiter(this, void 0, void 0, function* () {
        let resp = yield (yield theFetch(endpoint, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify({
                jsonrpc: "2.0",
                method,
                params,
                id: 0,
            }),
        })).json();
        if (resp.error) {
            throw new Error(resp.error.message);
        }
        return resp.result;
    });
}
export default function createRPCClient({ endpoint, }) {
    if (!endpoint) {
        throw new Error("RPC client requires an endpoint");
    }
    return {
        rpcFetch: (method, params) => rpcFetch(endpoint, method, params),
        subscribe: function ({ stateURI, keypath, txs, states, }) {
            return __awaiter(this, void 0, void 0, function* () {
                yield rpcFetch(endpoint, "RPC.Subscribe", {
                    stateURI,
                    txs,
                    states,
                    keypath,
                });
            });
        },
        identities: function () {
            return __awaiter(this, void 0, void 0, function* () {
                return (yield rpcFetch(endpoint, "RPC.Identities")).Identities.map(({ Address, Public }) => ({
                    address: Address,
                    public: Public,
                }));
            });
        },
        newIdentity: function () {
            return __awaiter(this, void 0, void 0, function* () {
                return (yield rpcFetch(endpoint, "RPC.NewIdentity"))
                    .Address;
            });
        },
        knownStateURIs: function () {
            return __awaiter(this, void 0, void 0, function* () {
                return (yield rpcFetch(endpoint, "RPC.KnownStateURIs"))
                    .StateURIs;
            });
        },
        sendTx: function (tx) {
            return __awaiter(this, void 0, void 0, function* () {
                yield rpcFetch(endpoint, "RPC.SendTx", { Tx: tx });
            });
        },
        addPeer: function ({ transportName, dialAddr, }) {
            return __awaiter(this, void 0, void 0, function* () {
                yield rpcFetch(endpoint, "RPC.AddPeer", {
                    TransportName: transportName,
                    DialAddr: dialAddr,
                });
            });
        },
        privateTreeMembers: function (stateURI) {
            return __awaiter(this, void 0, void 0, function* () {
                try {
                    return (yield rpcFetch(endpoint, "RPC.PrivateTreeMembers", {
                        StateURI: stateURI,
                    })).Members;
                }
                catch (err) {
                    if (err.toString().indexOf("no controller for that stateURI") >
                        -1) {
                        return [];
                    }
                    throw err;
                }
            });
        },
        peers: function () {
            return __awaiter(this, void 0, void 0, function* () {
                let peers = (yield rpcFetch(endpoint, "RPC.Peers")).Peers;
                return peers.map((peer) => ({
                    identities: (peer.Identities || []).map((i) => ({
                        address: i.Address,
                        signingPublicKey: i.SigningPublicKey,
                        encryptingPublicKey: i.EncryptingPublicKey,
                    })),
                    transport: peer.Transport,
                    dialAddr: peer.DialAddr,
                    stateURIs: peer.StateURIs,
                    lastContact: peer.LastContact > 0
                        ? new Date(peer.LastContact * 1000)
                        : null,
                }));
            });
        },
    };
}
