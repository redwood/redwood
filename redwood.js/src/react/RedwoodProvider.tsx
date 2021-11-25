import React, {
    createContext,
    useCallback,
    useState,
    useEffect,
    useRef,
} from "react";
import Redwood, {
    RedwoodClient,
    Identity,
    RPCIdentitiesResponse,
    PeersMap,
    RPCPeer,
    UnsubscribeFunc,
} from "..";

export interface IContext {
    identity: null | undefined | Identity;
    nodeIdentities: null | RPCIdentitiesResponse[];
    redwoodClient: null | RedwoodClient;
    httpHost: string;
    useWebsocket: boolean;
    subscribe: (stateURI: string) => Promise<UnsubscribeFunc>;
    subscribedStateURIs: React.MutableRefObject<{
        [stateURI: string]: boolean;
    }>;
    stateTrees: any;
    updateStateTree: (
        stateURI: string,
        newTree: any,
        newLeaves: string[]
    ) => void;
    updatePrivateTreeMembers: (stateURI: string, members: string[]) => void;
    leaves: { [txID: string]: boolean };
    privateTreeMembers: { [stateURI: string]: string[] };
    browserPeers: PeersMap;
    nodePeers: RPCPeer[];
    fetchIdentities: () => void;
    fetchRedwoodClient: () => void;
}

export const Context = createContext<IContext>({
    identity: null,
    nodeIdentities: null,
    redwoodClient: null,
    httpHost: "",
    useWebsocket: false,
    subscribe: (stateURI: string) => {
        return new Promise(() => {});
    },
    subscribedStateURIs: { current: {} },
    stateTrees: {},
    updateStateTree: (
        stateURI: string,
        newTree: any,
        newLeaves: string[]
    ) => {},
    updatePrivateTreeMembers: (stateURI: string, members: string[]) => {},
    leaves: {},
    privateTreeMembers: {},
    browserPeers: {},
    nodePeers: [],
    fetchIdentities: () => {},
    fetchRedwoodClient: () => {},
});

function Provider(props: {
    httpHost: string;
    rpcEndpoint?: string;
    useWebsocket?: boolean;
    webrtc?: boolean;
    identity?: Identity;
    children: React.ReactNode;
}) {
    let { httpHost, rpcEndpoint, useWebsocket, identity, webrtc, children } =
        props;

    const [nodeIdentities, setNodeIdentities] = useState<
        null | RPCIdentitiesResponse[]
    >(null);
    const [redwoodClient, setRedwoodClient] = useState<null | RedwoodClient>(
        null
    );
    const subscribedStateURIs = useRef<{ [stateURI: string]: boolean }>({});
    const [stateTrees, setStateTrees] = useState({});
    const [leaves, setLeaves] = useState({});
    const [browserPeers, setBrowserPeers] = useState({});
    const [privateTreeMembers, setPrivateTreeMembers] = useState({});
    const [nodePeers, setNodePeers] = useState<RPCPeer[]>([]);
    const [hasInitFetchedPI, setHasInitFetchedPI] = useState(false);
    const [error, setError] = useState(null);

    useEffect(() => {
        (async function () {
            subscribedStateURIs.current = {};
            setRedwoodClient(null);
            setNodeIdentities(null);
            setNodePeers([]);
            setStateTrees({});
            setLeaves({});
            setBrowserPeers({});
            setPrivateTreeMembers({});
            setError(null);

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

            fetchNodeIdentityPeers();
            setHasInitFetchedPI(true);

            setRedwoodClient(client);
        })();

        return () => {
            if (redwoodClient) {
                redwoodClient.close();
            }
        };
    }, [identity, httpHost, rpcEndpoint, webrtc]);

    const fetchNodeIdentityPeers = useCallback(async () => {
        if (!redwoodClient) {
            return;
        }

        if (!!redwoodClient.rpc) {
            try {
                let newNodeIdentities =
                    (await redwoodClient.rpc.identities()) || [];
                if (
                    newNodeIdentities.length !== (nodeIdentities || []).length
                ) {
                    setNodeIdentities(newNodeIdentities);
                }
            } catch (err) {
                console.error(err);
            }

            try {
                let newNodePeers = (await redwoodClient.rpc.peers()) || [];
                if (newNodePeers.length !== (nodePeers || []).length) {
                    setNodePeers(newNodePeers);
                }
            } catch (err) {
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
            if (hasInitFetchedPI) {
                await fetchNodeIdentityPeers();
            }
        }, 5000);

        return () => {
            console.log("restarted interval");
            clearInterval(intervalId);
        };
    }, [redwoodClient, nodePeers, nodeIdentities, hasInitFetchedPI]);

    let updatePrivateTreeMembers = useCallback(
        (stateURI: string, members: string[]) => {
            setPrivateTreeMembers((prevMembers) => ({
                ...prevMembers,
                [stateURI]: members,
            }));
        },
        [setStateTrees, setLeaves]
    );

    let updateStateTree = useCallback(
        (stateURI: string, newTree: any, newLeaves: string[]) => {
            setStateTrees((prevState) => ({
                ...prevState,
                [stateURI]: newTree,
            }));
            setLeaves((prevLeaves) => ({
                ...prevLeaves,
                [stateURI]: newLeaves,
            }));
        },
        [setStateTrees, setLeaves]
    );

    let subscribe = useCallback(
        async (stateURI: string) => {
            if (!redwoodClient) {
                return () => {};
            } else if (subscribedStateURIs.current[stateURI]) {
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
                        return;
                    }
                    let { stateURI, state, leaves } = next;
                    updateStateTree(stateURI, state, leaves);
                },
            });

            subscribedStateURIs.current[stateURI] = true;

            return () => {
                (async function () {
                    const unsubscribe = await unsubscribePromise;
                    unsubscribe();
                    subscribedStateURIs.current[stateURI] = false;
                })();
            };
        },
        [redwoodClient, useWebsocket, updateStateTree]
    );

    return (
        <Context.Provider
            value={{
                identity,
                nodeIdentities,
                redwoodClient,
                httpHost,
                useWebsocket: !!useWebsocket,
                subscribe,
                subscribedStateURIs,
                stateTrees,
                leaves,
                updateStateTree,
                updatePrivateTreeMembers,
                privateTreeMembers,
                browserPeers,
                nodePeers,
                fetchIdentities: () => {},
                fetchRedwoodClient: () => {},
            }}
        >
            {children}
        </Context.Provider>
    );
}

export default Provider;
