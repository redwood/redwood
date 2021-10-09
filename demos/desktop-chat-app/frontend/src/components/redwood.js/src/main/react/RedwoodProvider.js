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
exports.RedwoodContext = void 0;
const react_1 = __importStar(require("react"));
const reducers_1 = require("./reducers");
const state_tree_initial_state_1 = require("./reducers/state-tree.initial-state");
const __1 = __importDefault(require(".."));
exports.RedwoodContext = react_1.createContext({
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
    stateTrees: state_tree_initial_state_1.initialTreeState.stateTrees,
    leaves: state_tree_initial_state_1.initialTreeState.leaves,
    privateTreeMembers: state_tree_initial_state_1.initialTreeState.privateTreeMembers,
    getStateTree: () => { },
    browserPeers: {},
    nodePeers: [],
    fetchIdentities: () => { },
    fetchRedwoodClient: () => { },
    unsubscribeList: {},
});
function RedwoodProvider(props) {
    let { httpHost: httpHostProps = "", rpcEndpoint: rpcEndpointProps = "", useWebsocket = true, identity, webrtc, children, } = props;
    const [nodeIdentities, setNodeIdentities] = react_1.useState(null);
    const [redwoodClient, setRedwoodClient] = react_1.useState(null);
    const [browserPeers, setBrowserPeers] = react_1.useState({});
    const [httpHost, setHttpHost] = react_1.useState(httpHostProps);
    const [rpcEndpoint, setRpcEndpoint] = react_1.useState(rpcEndpointProps);
    const [nodePeers, setNodePeers] = react_1.useState([]);
    const [error, setError] = react_1.useState(null);
    const { actions: { updateStateTree: updateStateTreeAction, updateLeaves, updateTreeAndLeaves, updatePrivateTreeMembers: updatePrivateTreeMembersAction, resetTreeState, getStateTree: getStateTreeReducer, updateSubscribedStateURIs, updateUnsubscribeList, clearSubscribedStateURIs, }, reducer, state: { leaves, stateTrees, privateTreeMembers, subscribedStateURIs, unsubscribeList, }, dispatch, } = reducers_1.useTreeReducer();
    const runBatchUnsubscribe = react_1.useCallback(() => __awaiter(this, void 0, void 0, function* () {
        console.log(unsubscribeList, "un subbing");
        if (Object.values(unsubscribeList).length) {
            const unsub = yield Object.values(unsubscribeList)[0];
            console.log("unsubbing");
            unsub();
        }
    }), [unsubscribeList]);
    // If consumer changes httpHost or rpcEndpoint props override useState
    react_1.useEffect(() => {
        if (httpHostProps) {
            setHttpHost(httpHostProps);
        }
    }, [httpHostProps]);
    react_1.useEffect(() => {
        if (rpcEndpointProps) {
            setRpcEndpoint(rpcEndpointProps);
        }
    }, [rpcEndpointProps]);
    // useEvent("beforeunload", runBatchUnsubscribe, window, { capture: true });
    const resetState = react_1.useCallback(() => {
        setRedwoodClient(null);
        setNodeIdentities(null);
        setNodePeers([]);
        setBrowserPeers({});
        setError(null);
        dispatch(resetTreeState());
    }, []);
    react_1.useEffect(() => {
        (function () {
            return __awaiter(this, void 0, void 0, function* () {
                resetState();
                if (!httpHost) {
                    return;
                }
                let client = __1.default.createPeer({
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
    const fetchNodeIdentityPeers = react_1.useCallback(() => __awaiter(this, void 0, void 0, function* () {
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
    react_1.useEffect(() => {
        let intervalId = window.setInterval(() => __awaiter(this, void 0, void 0, function* () {
            yield fetchNodeIdentityPeers();
        }), 5000);
        return () => {
            clearInterval(intervalId);
        };
    }, [redwoodClient, nodePeers, httpHost, rpcEndpoint, nodeIdentities]);
    let getStateTree = react_1.useCallback((key, cb) => {
        dispatch(getStateTreeReducer({ key, cb }));
    }, []);
    let updatePrivateTreeMembers = react_1.useCallback((stateURI, members) => dispatch(updatePrivateTreeMembersAction({
        stateURI,
        members,
    })), []);
    let updateStateTree = react_1.useCallback((stateURI, newTree, newLeaves) => {
        dispatch(updateTreeAndLeaves({
            stateURI,
            newStateTree: newTree,
            newLeaves: newLeaves,
        }));
    }, [updateTreeAndLeaves]);
    let subscribe = react_1.useCallback((stateURI, subscribeCallback) => __awaiter(this, void 0, void 0, function* () {
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
    return (react_1.default.createElement(exports.RedwoodContext.Provider, { value: {
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
exports.default = RedwoodProvider;
