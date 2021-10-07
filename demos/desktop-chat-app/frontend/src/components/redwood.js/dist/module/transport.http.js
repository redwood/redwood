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
export default function (opts) {
    const { httpHost, onFoundPeers } = opts;
    let knownPeers = {};
    pollForPeers();
    let alreadyRespondedTo = {};
    let websocketConn;
    let websocketConnected = false;
    let websocketPendingSubscribeOpts = [];
    let unsubscribes = [];
    function close() {
        return __awaiter(this, void 0, void 0, function* () {
            for (let unsubscribe of unsubscribes) {
                unsubscribe();
            }
        });
    }
    function subscribe(opts) {
        return __awaiter(this, void 0, void 0, function* () {
            let { stateURI, keypath, fromTxID, states, txs, callback } = opts;
            try {
                let subscriptionType;
                if (states && txs) {
                    subscriptionType = "states,transactions";
                }
                else if (states) {
                    subscriptionType = "states";
                }
                else if (txs) {
                    subscriptionType = "transactions";
                }
                else {
                    throw new Error("must provide either `txs: true`, `states: true`, or both");
                }
                let unsubscribe;
                if (opts.useWebsocket) {
                    if (!websocketConn) {
                        let url = new URL(httpHost);
                        url.searchParams.set("state_uri", stateURI);
                        url.searchParams.set("keypath", keypath || "/");
                        url.searchParams.set("subscription_type", subscriptionType);
                        if (fromTxID) {
                            url.searchParams.set("from_tx", fromTxID);
                        }
                        url.protocol = "ws";
                        url.pathname = "/ws";
                        websocketConn = new WebSocket(url.toString());
                        websocketConn.onopen = function (evt) {
                            websocketConnected = true;
                            for (let pendingSubscribeOpts of websocketPendingSubscribeOpts) {
                                if (!websocketConn) {
                                    continue;
                                }
                                websocketConn.send(JSON.stringify({
                                    op: "subscribe",
                                    params: pendingSubscribeOpts,
                                }));
                            }
                        };
                        websocketConn.onclose = function (evt) {
                            websocketConnected = false;
                        };
                        websocketConn.onmessage = function (evt) {
                            let messages = evt.data
                                .split("\n")
                                .filter((x) => x.trim().length > 0);
                            for (let msg of messages) {
                                if (!websocketConn) {
                                    continue;
                                }
                                if (msg === "ping") {
                                    websocketConn.send("pong");
                                    continue;
                                }
                                try {
                                    let { stateURI, tx, state, leaves } = JSON.parse(msg);
                                    callback(null, { stateURI, tx, state, leaves });
                                }
                                catch (err) {
                                    callback(err, undefined);
                                }
                            }
                        };
                        unsubscribes.push(() => {
                            websocketConn === null || websocketConn === void 0 ? void 0 : websocketConn.close();
                            websocketConn = undefined;
                        });
                    }
                    else {
                        let subscribeOpts = {
                            stateURI,
                            keypath,
                            subscriptionType,
                            fromTxID,
                        };
                        if (websocketConnected) {
                            websocketConn.send(JSON.stringify({
                                op: "subscribe",
                                params: subscribeOpts,
                            }));
                        }
                        else {
                            websocketPendingSubscribeOpts.push(subscribeOpts);
                        }
                    }
                    unsubscribe = () => {
                        // if (websocketConn) {
                        //     websocketConn.close()
                        //     websocketConn = undefined
                        // }
                    };
                }
                else {
                    const headers = {
                        "State-URI": stateURI,
                        Accept: "application/json",
                        Subscribe: subscriptionType,
                    };
                    if (fromTxID) {
                        headers["From-Tx"] = fromTxID;
                    }
                    const resp = yield wrappedFetch(keypath || "/", {
                        method: "GET",
                        headers,
                    });
                    if (!resp.ok || !resp.body) {
                        callback("http transport: fetch failed", undefined);
                        return;
                    }
                    unsubscribe = readSubscription(stateURI, resp.body.getReader(), (err, update) => {
                        if (err) {
                            callback(err, undefined);
                            return;
                        }
                        let { stateURI, tx, state, leaves } = update;
                        if (tx) {
                            ack(tx.id);
                            if (!alreadyRespondedTo[tx.id]) {
                                alreadyRespondedTo[tx.id] = true;
                                callback(err, { stateURI, tx, state, leaves });
                            }
                        }
                        else {
                            callback(err, { stateURI, tx, state, leaves });
                        }
                    });
                }
                unsubscribes.push(unsubscribe);
                return unsubscribe;
            }
            catch (err) {
                callback("http transport: " + err, undefined);
                return () => { };
            }
        });
    }
    function readSubscription(stateURI, reader, callback) {
        let shouldStop = false;
        function unsubscribe() {
            shouldStop = true;
            reader.cancel();
        }
        setTimeout(() => __awaiter(this, void 0, void 0, function* () {
            try {
                const decoder = new TextDecoder("utf-8");
                let buffer = "";
                function read() {
                    return __awaiter(this, void 0, void 0, function* () {
                        const x = yield reader.read();
                        if (x.done) {
                            return;
                        }
                        const newData = decoder.decode(x.value);
                        buffer += newData;
                        let idx;
                        while ((idx = buffer.indexOf("\n")) > -1) {
                            if (shouldStop) {
                                return;
                            }
                            const line = buffer.substring(0, idx).trim();
                            if (line.length > 0) {
                                const payloadStr = line.substring(5).trim(); // remove "data:" prefix
                                let payload;
                                try {
                                    payload = JSON.parse(payloadStr);
                                }
                                catch (err) {
                                    console.error("Error parsing JSON:", payloadStr);
                                    callback("http transport: " + err, undefined);
                                    return;
                                }
                                callback(null, payload);
                            }
                            buffer = buffer.substring(idx + 1);
                        }
                        if (shouldStop) {
                            return;
                        }
                        read();
                    });
                }
                read();
            }
            catch (err) {
                callback("http transport: " + err, undefined);
                return;
            }
        }), 0);
        return unsubscribe;
    }
    function get({ stateURI, keypath, raw }) {
        return __awaiter(this, void 0, void 0, function* () {
            let url = keypath || "/";
            if (url.length > 0 && url[0] !== "/") {
                url = "/" + url;
            }
            if (raw) {
                url = url + "?raw=1";
            }
            return (yield (yield wrappedFetch(url, {
                headers: {
                    Accept: "application/json",
                    "State-URI": stateURI,
                },
            })).json());
        });
    }
    // @@TODO: private tx functionality
    function put(tx) {
        return __awaiter(this, void 0, void 0, function* () {
            let body;
            if (tx.attachment) {
                let fd;
                if (typeof window !== "undefined") {
                    fd = new FormData();
                }
                else {
                    let FormData = require("form-data");
                    fd = new FormData();
                }
                fd.append("attachment", tx.attachment);
                fd.append("patches", tx.patches.join("\n"));
                body = fd;
            }
            else {
                body = tx.patches.join("\n");
            }
            yield wrappedFetch("/", {
                method: "PUT",
                body: body,
                headers: {
                    "State-URI": tx.stateURI,
                    Version: tx.id,
                    Parents: (tx.parents || []).join(","),
                    Signature: tx.sig,
                    "Patch-Type": "braid",
                },
            });
        });
    }
    function ack(txID) {
        return __awaiter(this, void 0, void 0, function* () {
            yield wrappedFetch("/", {
                method: "ACK",
                body: txID,
            });
        });
    }
    function storeBlob(file) {
        return __awaiter(this, void 0, void 0, function* () {
            let formData;
            if (typeof window !== "undefined") {
                formData = new FormData();
                formData.append("blob", file);
            }
            else {
                let FormData = require("form-data");
                formData = new FormData();
                formData.append("blob", file);
            }
            const resp = yield wrappedFetch(`/`, {
                method: "POST",
                headers: {
                    Blob: "true",
                },
                body: formData,
            });
            return yield resp.json();
        });
    }
    function authorize(identity) {
        return __awaiter(this, void 0, void 0, function* () {
            const resp = yield wrappedFetch(`/`, {
                method: "AUTHORIZE",
            });
            const challengeHex = yield resp.text();
            const challenge = Buffer.from(challengeHex, "hex");
            const sigHex = identity.signBytes(challenge);
            const resp2 = yield wrappedFetch(`/`, {
                method: "AUTHORIZE",
                headers: {
                    Response: sigHex,
                },
            });
        });
    }
    let cookies = {};
    function wrappedFetch(path, options) {
        return __awaiter(this, void 0, void 0, function* () {
            if (typeof window === "undefined") {
                // We have to manually parse and set cookies because isomorphic-fetch doesn't do it for us
                let cookieStr = Object.keys(cookies)
                    .map((cookieName) => `${cookieName}=${cookies[cookieName]}`)
                    .join(";");
                options.headers = Object.assign(Object.assign(Object.assign({}, makeRequestHeaders()), options.headers), { Cookie: cookieStr });
            }
            else {
                options.headers = Object.assign(Object.assign({}, makeRequestHeaders()), options.headers);
            }
            options.credentials = "include";
            path = path || "";
            if (path[0] !== "/") {
                path = "/" + (path || "");
            }
            let url = !httpHost ? path : httpHost + path;
            const resp = yield theFetch(url, options);
            if (!resp.ok) {
                let text = yield resp.text();
                throw { statusCode: resp.status, error: text };
            }
            if (typeof window === "undefined") {
                // Manual cookie parsing
                let rawHeaders = resp.headers.raw();
                for (let str of rawHeaders["set-cookie"] || []) {
                    let keyVal = str.substr(0, str.indexOf(";")).split("=");
                    cookies[keyVal[0]] = keyVal[1];
                }
            }
            // Receive list of peers from the Alt-Svc header
            const altSvcHeader = resp.headers.get("Alt-Svc");
            if (altSvcHeader) {
                const peers = {};
                const peerHeaders = altSvcHeader.split(",").map((x) => x.trim());
                for (let peer of peerHeaders) {
                    const x = peer.match(/^\s*(\w+)="([^"]+)"/);
                    if (!x) {
                        continue;
                    }
                    const tptName = x[1];
                    const reachableAt = x[2];
                    peers[tptName] = peers[tptName] || {};
                    peers[tptName][reachableAt] = true;
                }
                if (onFoundPeers) {
                    onFoundPeers(peers);
                }
            }
            return resp;
        });
    }
    function pollForPeers() {
        setInterval(() => __awaiter(this, void 0, void 0, function* () {
            try {
                yield wrappedFetch(`/`, { method: "HEAD" });
            }
            catch (err) {
                console.error("pollForPeers error ~>", err);
            }
        }), 5000);
    }
    function makeRequestHeaders() {
        const headers = {};
        const altSvc = [];
        for (let tptName of Object.keys(knownPeers)) {
            for (let reachableAt of Object.keys(knownPeers[tptName])) {
                altSvc.push(`${tptName}="${reachableAt}"`);
            }
        }
        if (altSvc.length > 0) {
            headers["Alt-Svc"] = altSvc.join(", ");
        }
        return headers;
    }
    function foundPeers(peers) {
        knownPeers = peers;
    }
    return {
        transportName: () => "http",
        altSvcAddresses: () => [],
        subscribe,
        get,
        put,
        ack,
        storeBlob,
        authorize,
        foundPeers,
        close,
    };
}
