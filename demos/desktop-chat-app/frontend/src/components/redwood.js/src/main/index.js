"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    Object.defineProperty(o, k2, { enumerable: true, get: function() { return m[k]; } });
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || function (mod) {
    if (mod && mod.__esModule) return mod;
    var result = {};
    if (mod != null) for (var k in mod) if (k !== "default" && Object.prototype.hasOwnProperty.call(mod, k)) __createBinding(result, mod, k);
    __setModuleDefault(result, mod);
    return result;
};
var __exportStar = (this && this.__exportStar) || function(m, exports) {
    for (var p in m) if (p !== "default" && !Object.prototype.hasOwnProperty.call(exports, p)) __createBinding(exports, m, p);
};
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const identity = __importStar(require("./identity"));
const sync9 = __importStar(require("./resolver.sync9.browser"));
const dumb = __importStar(require("./resolver.dumb.browser"));
const utils = __importStar(require("./utils"));
const transport_http_1 = __importDefault(require("./transport.http"));
// import * as webrtcTransport from './transport.webrtc'
const transport_rpc_1 = __importDefault(require("./transport.rpc"));
__exportStar(require("./types"), exports);
exports.default = {
    createPeer,
    // submodules
    identity,
    sync9,
    dumb,
    utils,
};
function createPeer(opts) {
    const { httpHost, identity, webrtc, onFoundPeersCallback, rpcEndpoint } = opts;
    const http = transport_http_1.default({ onFoundPeers, httpHost });
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
        rpc: rpcEndpoint ? transport_rpc_1.default({ endpoint: rpcEndpoint }) : undefined,
        close,
    };
}
