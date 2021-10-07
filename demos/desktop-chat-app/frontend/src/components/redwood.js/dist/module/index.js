var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
import * as identity from "./identity";
import * as sync9 from "./resolver.sync9.browser";
import * as dumb from "./resolver.dumb.browser";
import * as utils from "./utils";
import httpTransport from "./transport.http";
// import * as webrtcTransport from './transport.webrtc'
import rpcTransport from "./transport.rpc";
export * from "./types";
export default {
    createPeer,
    // submodules
    identity,
    sync9,
    dumb,
    utils,
};
function createPeer(opts) {
    const { httpHost, identity, webrtc, onFoundPeersCallback, rpcEndpoint } = opts;
    const http = httpTransport({ onFoundPeers, httpHost });
    const transports = [http];
    // if (webrtc === true) {
    //     transports.push(webrtcTransport({ onFoundPeers, peerID: identity.peerID }))
    // }
    let knownPeers = {};
    function onFoundPeers(peers) {
        knownPeers = utils.deepmerge(knownPeers, peers);
        transports.forEach((tpt) => tpt.foundPeers(knownPeers));
        if (onFoundPeersCallback) {
            onFoundPeersCallback(knownPeers);
        }
    }
    function peers() {
        return __awaiter(this, void 0, void 0, function* () {
            return knownPeers;
        });
    }
    function authorize() {
        return __awaiter(this, void 0, void 0, function* () {
            if (!identity) {
                throw new Error("cannot .authorize() without an identity");
            }
            for (let tpt of transports) {
                if (tpt.authorize) {
                    yield tpt.authorize(identity);
                }
            }
        });
    }
    function subscribe(params) {
        return __awaiter(this, void 0, void 0, function* () {
            let unsubscribers = (yield Promise.all(transports.map((tpt) => tpt.subscribe(params)))).filter((unsub) => !!unsub);
            return () => {
                for (let unsub of unsubscribers) {
                    if (unsub) {
                        unsub();
                    }
                }
            };
        });
    }
    function get({ stateURI, keypath, raw }) {
        return __awaiter(this, void 0, void 0, function* () {
            for (let tpt of transports) {
                if (tpt.get) {
                    return tpt.get({ stateURI, keypath, raw });
                }
            }
        });
    }
    function put(tx) {
        return __awaiter(this, void 0, void 0, function* () {
            if (!identity) {
                throw new Error("cannot .put() without an identity");
            }
            tx.from = identity.address;
            tx.sig = identity.signTx(tx);
            for (let tpt of transports) {
                if (tpt.put) {
                    try {
                        yield tpt.put(tx);
                    }
                    catch (err) {
                        console.error("error PUTting to peer ~>", err);
                    }
                }
            }
        });
    }
    function storeBlob(file) {
        return __awaiter(this, void 0, void 0, function* () {
            for (let tpt of transports) {
                if (tpt.storeBlob) {
                    return tpt.storeBlob(file);
                }
            }
            throw new Error("no transports support storeBlob");
        });
    }
    function close() {
        return __awaiter(this, void 0, void 0, function* () {
            for (let tpt of transports) {
                yield tpt.close();
            }
        });
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
    };
}
