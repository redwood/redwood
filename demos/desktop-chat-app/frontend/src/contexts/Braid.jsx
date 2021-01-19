import React, { createContext, useCallback, useState, useEffect } from 'react'
import rpcFetch from '../utils/rpcFetch'
import * as Braid from '../braidjs/braid-src'

export const Context = createContext({
    nodeAddress: '',
    registry: [],
    subscribedStateURIs: {},
    appState: {},
    leaves: {},
    knownPeers: {},
})

function Provider({ children }) {
    const [braidClient, setBraidClient] = useState(null)
    const [registry, setRegistry] = useState({})
    const [appState, setAppState] = useState({})
    const [leaves, setLeaves] = useState({})
    const [knownPeers, setKnownPeers] = useState({})
    const [nodeAddress, setNodeAddress] = useState(null)
    const [subscribedStateURIs, setSubscribedStateURIs] = useState({})
    const [error, setError] = useState(null)

    useEffect(() => {
        (async function() {
            let addr = await rpcFetch('RPC.NodeAddress', {})
            setNodeAddress(addr)

            let braidClient = Braid.createPeer({
                identity: Braid.identity.random(),
                httpHost: 'http://localhost:8080',
                webrtc: false,
                onFoundPeersCallback: (peers) => {
                    setKnownPeers(peers)
                },
            })
            await braidClient.authorize()
            await braidClient.subscribeStates('chat.redwood.dev/registry', '/', async (err, { state: newRegistry }) => {
                if (err) {
                    console.error(err)
                    return
                }
                setRegistry(newRegistry)
            })
            setBraidClient(braidClient)
        })()
    }, [])

    useEffect(() => {
        if (!braidClient || !registry.rooms) {
            return
        }
        for (let stateURI of registry.rooms) {
            (function (stateURI) {
                if (!subscribedStateURIs[stateURI]) {
                    braidClient.subscribeStates(stateURI, '/', async (err, update) => {
                        console.log(stateURI, update)
                        if (err) {
                            setError(err)
                            console.error(err)
                            return
                        }
                        let { state: newState, leaves: newLeaves } = update
                        setSubscribedStateURIs(prevState => ({ ...prevState, [stateURI]: true }))
                        setAppState(prevState => ({ ...prevState, [stateURI]: newState }))
                        setLeaves(prevState => ({ ...prevState, [stateURI]: newLeaves }))
                    })
                }
            })(stateURI)
        }
    }, [braidClient, registry, subscribedStateURIs])

    return (
      <Context.Provider value={{
          registry,
          appState,
          nodeAddress,
          leaves,
          subscribedStateURIs,
          knownPeers,
      }}>
          {children}
      </Context.Provider>
    )
}

export default Provider