import React, { createContext, useCallback, useState, useEffect, useRef } from 'react'
import rpcFetch from '../utils/rpcFetch'
import Redwood from '../redwood.js'

export const Context = createContext({
    nodeAddress: null,
    redwoodClient: null,
    subscribedStateURIs: {},
    stateTrees: {},
    updateStateTree: () => {},
    leaves: {},
    knownPeers: {},
})

function Provider({ children }) {
    const [nodeAddress, setNodeAddress] = useState(null)
    const [redwoodClient, setRedwoodClient] = useState(null)
    const subscribedStateURIs = useRef({})
    const [stateTrees, setStateTrees] = useState({})
    const [leaves, setLeaves] = useState({})
    const [knownPeers, setKnownPeers] = useState({})
    const [error, setError] = useState(null)

    useEffect(() => {
        (async function() {
            let addr = await rpcFetch('RPC.NodeAddress', {})
            setNodeAddress(addr.Address)
        })()
    }, [])

    useEffect(() => {
        if (!nodeAddress) {
            return
        }
        ;(async function() {
            let redwoodClient = Redwood.createPeer({
                identity: Redwood.identity.random(),
                httpHost: 'http://localhost:8080',
                webrtc: false,
                onFoundPeersCallback: (peers) => { setKnownPeers(peers) },
            })
            await redwoodClient.authorize()
            setRedwoodClient(redwoodClient)
        })()
    }, [nodeAddress])

    let updateStateTree = useCallback((stateURI, newTree, newLeaves) => {
        setStateTrees(prevState => ({ ...prevState, [stateURI]: newTree }))
        setLeaves(prevLeaves => ({ ...prevLeaves, [stateURI]: newLeaves }))
    }, [setStateTrees])

    return (
      <Context.Provider value={{
          nodeAddress,
          redwoodClient,
          subscribedStateURIs,
          stateTrees,
          leaves,
          updateStateTree,
          knownPeers,
      }}>
          {children}
      </Context.Provider>
    )
}

export default Provider