import React, {
    createContext,
    useCallback,
    useState,
    useEffect,
    useRef,
} from "react";
import { useEvent } from "react-use";

import { useTreeReducer } from "./reducers";
import { initialTreeState } from "./reducers/state-tree.initial-state";
import {
    PrivateTreeMembersObj,
    SubscribedStateURIsObj,
    UnsubscribeListObj,
} from "./reducers/state-tree.type";

import Redwood, {
    RedwoodClient,
    Identity,
    RPCIdentitiesResponse,
    PeersMap,
    RPCPeer,
} from "..";

export interface IContext {
    identity: null | undefined | Identity;
    nodeIdentities: null | RPCIdentitiesResponse[];
    redwoodClient: null | RedwoodClient;
    httpHost: string;
    setHttpHost: (httpHost: string) => void;
    rpcEndpoint: string;
    setRpcEndpoint: (rpcEndpoint: string) => void;
    useWebsocket: boolean;
    subscribe: (
        stateURI: string,
        subscribeCallback: (err: any, data: any) => void
    ) => void;
    subscribedStateURIs: SubscribedStateURIsObj;
    updateStateTree: (
        stateURI: string,
        newTree: any,
        newLeaves: string[]
    ) => void;
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

export const RedwoodContext = createContext<IContext>({
    identity: null,
    nodeIdentities: null,
    redwoodClient: null,
    httpHost: "",
    setHttpHost: () => {},
    rpcEndpoint: "",
    setRpcEndpoint: () => {},
    useWebsocket: true,
    subscribe: (
        stateURI: string,
        subscribeCallback: (err: any, data: any) => void
    ) => {
        return new Promise(() => {});
    },
    subscribedStateURIs: {},
    updateStateTree: (
        stateURI: string,
        newTree: any,
        newLeaves: string[]
    ) => {},
    updatePrivateTreeMembers: (stateURI: string, members: string[]) => {},
    stateTrees: initialTreeState.stateTrees,
    leaves: initialTreeState.leaves,
    privateTreeMembers: initialTreeState.privateTreeMembers,
    getStateTree: () => {},
    browserPeers: {},
    nodePeers: [],
    fetchIdentities: () => {},
    fetchRedwoodClient: () => {},
    unsubscribeList: {},
});

function RedwoodProvider(props: {
    httpHost?: string;
    rpcEndpoint?: string;
    useWebsocket?: boolean;
    webrtc?: boolean;
    identity?: Identity;
    children: React.ReactNode;
}) {
    let {
        httpHost: httpHostProps = "",
        rpcEndpoint: rpcEndpointProps = "",
        useWebsocket = true,
        identity,
        webrtc,
        children,
    } = props;

    const [nodeIdentities, setNodeIdentities] = useState<
        null | RPCIdentitiesResponse[]
    >(null);
    const [redwoodClient, setRedwoodClient] = useState<null | RedwoodClient>(
        null
    );
    const [browserPeers, setBrowserPeers] = useState({});
    const [httpHost, setHttpHost] = useState(httpHostProps);
    const [rpcEndpoint, setRpcEndpoint] = useState(rpcEndpointProps);
    const [nodePeers, setNodePeers] = useState<RPCPeer[]>([]);
    const [error, setError] = useState(null);
    const {
        actions: {
            updateStateTree: updateStateTreeAction,
            updateLeaves,
            updateTreeAndLeaves,
            updatePrivateTreeMembers: updatePrivateTreeMembersAction,
            resetTreeState,
            getStateTree: getStateTreeReducer,
            updateSubscribedStateURIs,
            updateUnsubscribeList,
            clearSubscribedStateURIs,
        },
        reducer,
        state: {
            leaves,
            stateTrees,
            privateTreeMembers,
            subscribedStateURIs,
            unsubscribeList,
        },
        dispatch,
    } = useTreeReducer();

    const runBatchUnsubscribe = useCallback(async () => {
        console.log(unsubscribeList, "un subbing");

        if (Object.values(unsubscribeList).length) {
            const unsub = await Object.values(unsubscribeList)[0];
            console.log("unsubbing");
            unsub();
        }
    }, [unsubscribeList]);

    // If consumer changes httpHost or rpcEndpoint props override useState
    useEffect(() => {
        if (httpHostProps) {
            setHttpHost(httpHostProps);
        }
    }, [httpHostProps]);

    useEffect(() => {
        if (rpcEndpointProps) {
            setRpcEndpoint(rpcEndpointProps);
        }
    }, [rpcEndpointProps]);

    // useEvent("beforeunload", runBatchUnsubscribe, window, { capture: true });

    const resetState = useCallback(() => {
        setRedwoodClient(null);
        setNodeIdentities(null);
        setNodePeers([]);
        setBrowserPeers({});
        setError(null);
        dispatch(resetTreeState());
    }, []);

    useEffect(() => {
        (async function () {
            resetState();

            if (!httpHost) {
                return;
            }
            let client = Redwood.createPeer({
                identity,
                httpHost,
                rpcEndpoint,
                webrtc,
                onFoundPeersCallback: (peers) => {
                    setBrowserPeers(peers);
                },
            });

            if (!!identity) {
                await client.authorize();
            }
            
            if (!!rpcEndpoint && !!client.rpc) {
                let ucan = await client.rpc.ucan()
                client.setUcan(ucan)
            }

            setRedwoodClient(client);
        })();

        return () => {
            if (redwoodClient) {
                console.log("closed");
                redwoodClient.close();
            }
        };
    }, [identity, httpHost, rpcEndpoint, webrtc, setHttpHost, setRpcEndpoint]);

    const fetchNodeIdentityPeers = useCallback(async () => {
        if (!redwoodClient) {
            return;
        }

        console.log("ran");

        if (!!redwoodClient.rpc) {
            try {
                let newNodeIdentities =
                    (await redwoodClient.rpc.identities()) || [];
                if (
                    newNodeIdentities.length !== (nodeIdentities || []).length
                ) {
                    setNodeIdentities(newNodeIdentities);
                }
            } catch (err: any) {
                console.error(err);
            }

            try {
                let newNodePeers = (await redwoodClient.rpc.peers()) || [];
                if (newNodePeers.length !== (nodePeers || []).length) {
                    setNodePeers(newNodePeers);
                }
            } catch (err: any) {
                console.error(err);
            }
        }
    }, [
        redwoodClient,
        setNodePeers,
        setNodeIdentities,
        nodeIdentities,
        nodePeers,
    ]);

    useEffect(() => {
        let intervalId = window.setInterval(async () => {
            await fetchNodeIdentityPeers();
        }, 5000);

        return () => {
            clearInterval(intervalId);
        };
    }, [redwoodClient, nodePeers, httpHost, rpcEndpoint, nodeIdentities]);

    let getStateTree = useCallback((key, cb) => {
        dispatch(getStateTreeReducer({ key, cb }));
    }, []);

    let updatePrivateTreeMembers = useCallback(
        (stateURI: string, members: string[]) =>
            dispatch(
                updatePrivateTreeMembersAction({
                    stateURI,
                    members,
                })
            ),
        []
    );

    let updateStateTree = useCallback(
        (stateURI: string, newTree: any, newLeaves: string[]) => {
            dispatch(
                updateTreeAndLeaves({
                    stateURI,
                    newStateTree: newTree,
                    newLeaves: newLeaves,
                })
            );
        },
        [updateTreeAndLeaves]
    );

    let subscribe = useCallback(
        async (
            stateURI: string,
            subscribeCallback: (err: any, data: any) => void
        ) => {
            if (!redwoodClient) {
                return () => {};
            } else if (subscribedStateURIs[stateURI]) {
                return () => {};
            }
            const unsubscribePromise = redwoodClient.subscribe({
                stateURI,
                keypath: "/",
                states: true,
                useWebsocket,
                callback: async (err, next) => {
                    if (err) {
                        console.error(err);
                        subscribeCallback(err, null);
                        return;
                    }
                    let { stateURI: nextStateURI, state, leaves } = next;
                    updateStateTree(nextStateURI, state, leaves);
                    subscribeCallback(false, next);
                },
            });

            dispatch(
                updateSubscribedStateURIs({ stateURI, isSubscribed: true })
            );

            dispatch(
                updateUnsubscribeList({
                    stateURI,
                    unsub: unsubscribePromise,
                })
            );
        },
        [redwoodClient, useWebsocket, updateStateTree, subscribedStateURIs]
    );

    return (
        <RedwoodContext.Provider
            value={{
                identity,
                nodeIdentities,
                redwoodClient,
                httpHost,
                rpcEndpoint,
                setHttpHost,
                setRpcEndpoint,
                useWebsocket: !!useWebsocket,
                subscribe,
                subscribedStateURIs,
                stateTrees,
                leaves,
                privateTreeMembers,
                updateStateTree,
                updatePrivateTreeMembers,
                getStateTree,
                browserPeers,
                nodePeers,
                fetchIdentities: () => {},
                fetchRedwoodClient: () => {},
                unsubscribeList,
            }}
        >
            {children}
        </RedwoodContext.Provider>
    );
}

export default RedwoodProvider;
