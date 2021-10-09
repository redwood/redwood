import React from "react";
import { PrivateTreeMembersObj, SubscribedStateURIsObj, UnsubscribeListObj } from "./reducers/state-tree.type";
import { RedwoodClient, Identity, RPCIdentitiesResponse, PeersMap, RPCPeer } from "..";
export interface IContext {
    identity: null | undefined | Identity;
    nodeIdentities: null | RPCIdentitiesResponse[];
    redwoodClient: null | RedwoodClient;
    httpHost: string;
    setHttpHost: (httpHost: string) => void;
    rpcEndpoint: string;
    setRpcEndpoint: (rpcEndpoint: string) => void;
    useWebsocket: boolean;
    subscribe: (stateURI: string, subscribeCallback: (err: any, data: any) => void) => void;
    subscribedStateURIs: SubscribedStateURIsObj;
    updateStateTree: (stateURI: string, newTree: any, newLeaves: string[]) => void;
    updatePrivateTreeMembers: (stateURI: string, members: string[]) => void;
    stateTrees: any;
    leaves: Object;
    privateTreeMembers: PrivateTreeMembersObj;
    browserPeers: PeersMap;
    nodePeers: RPCPeer[];
    fetchIdentities: () => void;
    fetchRedwoodClient: () => void;
    getStateTree: any;
    unsubscribeList: UnsubscribeListObj;
}
export declare const RedwoodContext: React.Context<IContext>;
declare function RedwoodProvider(props: {
    httpHost?: string;
    rpcEndpoint?: string;
    useWebsocket?: boolean;
    webrtc?: boolean;
    identity?: Identity;
    children: React.ReactNode;
}): JSX.Element;
export default RedwoodProvider;
//# sourceMappingURL=RedwoodProvider.d.ts.map