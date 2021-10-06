import * as identity from "./identity";
import * as sync9 from "./resolver.sync9.browser";
import * as dumb from "./resolver.dumb.browser";
import * as utils from "./utils";
import { Identity, PeersMap, PeersCallback, SubscribeParams, GetParams, Tx, StoreBlobResponse } from "./types";
export * from "./types";
declare const _default: {
    createPeer: typeof createPeer;
    identity: typeof identity;
    sync9: typeof sync9;
    dumb: typeof dumb;
    utils: typeof utils;
};
export default _default;
interface CreatePeerOptions {
    httpHost: string;
    identity?: Identity;
    webrtc?: boolean;
    onFoundPeersCallback?: PeersCallback;
    rpcEndpoint?: string;
}
declare function createPeer(opts: CreatePeerOptions): {
    identity: Identity | undefined;
    get: ({ stateURI, keypath, raw }: GetParams) => Promise<any>;
    subscribe: (params: SubscribeParams) => Promise<() => void>;
    put: (tx: Tx) => Promise<void>;
    storeBlob: (file: string | Blob) => Promise<StoreBlobResponse>;
    authorize: () => Promise<void>;
    peers: () => Promise<PeersMap>;
    rpc: import("./types").RPCClient | undefined;
    close: () => Promise<void>;
};
//# sourceMappingURL=index.d.ts.map