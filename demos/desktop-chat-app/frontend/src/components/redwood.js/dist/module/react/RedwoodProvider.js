var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
import React, { createContext, useCallback, useState, useEffect } from "react";
import { useTreeReducer } from "./reducers";
import { initialTreeState } from "./reducers/state-tree.initial-state";
import Redwood from "..";
export const RedwoodContext = createContext({
    identity: null,
    nodeIdentities: null,
    redwoodClient: null,
    httpHost: "",
    setHttpHost: () => { },
    rpcEndpoint: "",
    setRpcEndpoint: () => { },
    useWebsocket: true,
    subscribe: (stateURI, subscribeCallback) => {
        return new Promise(() => { });
    },
    subscribedStateURIs: {},
    updateStateTree: (stateURI, newTree, newLeaves) => { },
    updatePrivateTreeMembers: (stateURI, members) => { },
    stateTrees: initialTreeState.stateTrees,
    leaves: initialTreeState.leaves,
    privateTreeMembers: initialTreeState.privateTreeMembers,
    getStateTree: () => { },
    browserPeers: {},
    nodePeers: [],
    fetchIdentities: () => { },
    fetchRedwoodClient: () => { },
    unsubscribeList: {},
});
function RedwoodProvider(props) {
    let { httpHost: httpHostProps = "", rpcEndpoint: rpcEndpointProps = "", useWebsocket = true, identity, webrtc, children, } = props;
    const [nodeIdentities, setNodeIdentities] = useState(null);
    const [redwoodClient, setRedwoodClient] = useState(null);
    const [browserPeers, setBrowserPeers] = useState({});
    const [httpHost, setHttpHost] = useState(httpHostProps);
    const [rpcEndpoint, setRpcEndpoint] = useState(rpcEndpointProps);
    const [nodePeers, setNodePeers] = useState([]);
    const [error, setError] = useState(null);
    const { actions: { updateStateTree: updateStateTreeAction, updateLeaves, updateTreeAndLeaves, updatePrivateTreeMembers: updatePrivateTreeMembersAction, resetTreeState, getStateTree: getStateTreeReducer, updateSubscribedStateURIs, updateUnsubscribeList, clearSubscribedStateURIs, }, reducer, state: { leaves, stateTrees, privateTreeMembers, subscribedStateURIs, unsubscribeList, }, dispatch, } = useTreeReducer();
    const runBatchUnsubscribe = useCallback(() => __awaiter(this, void 0, void 0, function* () {
        console.log(unsubscribeList, "un subbing");
        if (Object.values(unsubscribeList).length) {
            const unsub = yield Object.values(unsubscribeList)[0];
            console.log("unsubbing");
            unsub();
        }
    }), [unsubscribeList]);
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
        (function () {
            return __awaiter(this, void 0, void 0, function* () {
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
                    yield client.authorize();
                }
                setRedwoodClient(client);
            });
        })();
        return () => {
            if (redwoodClient) {
                console.log("closed");
                redwoodClient.close();
            }
        };
    }, [identity, httpHost, rpcEndpoint, webrtc, setHttpHost, setRpcEndpoint]);
    const fetchNodeIdentityPeers = useCallback(() => __awaiter(this, void 0, void 0, function* () {
        if (!redwoodClient) {
            return;
        }
        console.log("ran");
        if (!!redwoodClient.rpc) {
            try {
                let newNodeIdentities = (yield redwoodClient.rpc.identities()) || [];
                if (newNodeIdentities.length !== (nodeIdentities || []).length) {
                    setNodeIdentities(newNodeIdentities);
                }
            }
            catch (err) {
                console.error(err);
            }
            try {
                let newNodePeers = (yield redwoodClient.rpc.peers()) || [];
                if (newNodePeers.length !== (nodePeers || []).length) {
                    setNodePeers(newNodePeers);
                }
            }
            catch (err) {
                console.error(err);
            }
        }
    }), [
        redwoodClient,
        setNodePeers,
        setNodeIdentities,
        nodeIdentities,
        nodePeers,
    ]);
    useEffect(() => {
        let intervalId = window.setInterval(() => __awaiter(this, void 0, void 0, function* () {
            yield fetchNodeIdentityPeers();
        }), 5000);
        return () => {
            clearInterval(intervalId);
        };
    }, [redwoodClient, nodePeers, httpHost, rpcEndpoint, nodeIdentities]);
    let getStateTree = useCallback((key, cb) => {
        dispatch(getStateTreeReducer({ key, cb }));
    }, []);
    let updatePrivateTreeMembers = useCallback((stateURI, members) => dispatch(updatePrivateTreeMembersAction({
        stateURI,
        members,
    })), []);
    let updateStateTree = useCallback((stateURI, newTree, newLeaves) => {
        dispatch(updateTreeAndLeaves({
            stateURI,
            newStateTree: newTree,
            newLeaves: newLeaves,
        }));
    }, [updateTreeAndLeaves]);
    let subscribe = useCallback((stateURI, subscribeCallback) => __awaiter(this, void 0, void 0, function* () {
        if (!redwoodClient) {
            return () => { };
        }
        else if (subscribedStateURIs[stateURI]) {
            return () => { };
        }
        const unsubscribePromise = redwoodClient.subscribe({
            stateURI,
            keypath: "/",
            states: true,
            useWebsocket,
            callback: (err, next) => __awaiter(this, void 0, void 0, function* () {
                if (err) {
                    console.error(err);
                    subscribeCallback(err, null);
                    return;
                }
                let { stateURI: nextStateURI, state, leaves } = next;
                updateStateTree(nextStateURI, state, leaves);
                subscribeCallback(false, next);
            }),
        });
        dispatch(updateSubscribedStateURIs({ stateURI, isSubscribed: true }));
        dispatch(updateUnsubscribeList({
            stateURI,
            unsub: unsubscribePromise,
        }));
    }), [redwoodClient, useWebsocket, updateStateTree, subscribedStateURIs]);
    return (React.createElement(RedwoodContext.Provider, { value: {
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
            fetchIdentities: () => { },
            fetchRedwoodClient: () => { },
            unsubscribeList,
        } }, children));
}
export default RedwoodProvider;
