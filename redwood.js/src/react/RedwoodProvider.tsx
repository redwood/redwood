import React, { createContext, useCallback, useState, useEffect, useRef } from 'react'
import Redwood, { RedwoodClient, Identity, RPCIdentitiesResponse, PeersMap } from '..'

export interface IContext {
    identity: null | undefined | Identity
    nodeIdentities: null | RPCIdentitiesResponse[]
    redwoodClient: null | RedwoodClient
    httpHost: string
    useWebsocket: boolean
    subscribedStateURIs: React.MutableRefObject<{[stateURI: string]: boolean}>
    stateTrees: any
    updateStateTree: (stateURI: string, newTree: any, newLeaves: string[]) => void
    leaves: {[txID: string]: boolean}
    knownPeers: PeersMap,
    fetchIdentities: any,
}

export const Context = createContext<IContext>({
    identity: null,
    nodeIdentities: null,
    redwoodClient: null,
    httpHost: '',
    useWebsocket: false,
    subscribedStateURIs: { current: {} },
    stateTrees: {},
    updateStateTree: (stateURI: string, newTree: any, newLeaves: string[]) => {},
    leaves: {},
    knownPeers: {},
    fetchIdentities: () => {},
})

function Provider(props: {
    httpHost: string,
    rpcEndpoint?: string,
    useWebsocket?: boolean,
    webrtc?: boolean,
    identity?: Identity,
    children: React.ReactNode,
}) {
    let { httpHost, rpcEndpoint, useWebsocket, identity, webrtc, children } = props

    const [nodeIdentities, setNodeIdentities] = useState<null|RPCIdentitiesResponse[]>(null)
    const [redwoodClient, setRedwoodClient] = useState<null|RedwoodClient>(null)
    const subscribedStateURIs = useRef<{[stateURI: string]: boolean}>({})
    const [stateTrees, setStateTrees] = useState({})
    const [leaves, setLeaves] = useState({})
    const [knownPeers, setKnownPeers] = useState({})
    const [error, setError] = useState(null)

    let fetchIdentities = async (redwoodClient: any) => {
      console.log('WORKING', redwoodClient.rpc)
      let nodeIdentities = await redwoodClient.rpc.identities()	
      setNodeIdentities(nodeIdentities)
    }	

    useEffect(() => {
        ;(async function() {
            let redwoodClient = Redwood.createPeer({
                identity,
                httpHost,
                rpcEndpoint,
                webrtc,
                onFoundPeersCallback: (peers) => { setKnownPeers(peers) },
            })
            if (!!identity) {
                await redwoodClient.authorize()
            }

            await fetchIdentities(redwoodClient)

            setRedwoodClient(redwoodClient)
            
        })()
    }, [identity, httpHost, rpcEndpoint, webrtc])

    let updateStateTree = useCallback((stateURI: string, newTree: any, newLeaves: string[]) => {
        setStateTrees(prevState => ({ ...prevState, [stateURI]: newTree }))
        setLeaves(prevLeaves => ({ ...prevLeaves, [stateURI]: newLeaves }))
    }, [setStateTrees])

    return (
      <Context.Provider value={{
          identity,
          nodeIdentities,
          redwoodClient,
          httpHost,
          useWebsocket: !!useWebsocket,
          subscribedStateURIs,
          stateTrees,
          leaves,
          updateStateTree,
          knownPeers,
          fetchIdentities,
      }}>
          {children}
      </Context.Provider>
    )
}

export default Provider