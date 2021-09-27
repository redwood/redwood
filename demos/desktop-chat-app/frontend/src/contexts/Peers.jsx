import React, { createContext, useState, useEffect } from 'react'
import { useRedwood } from '@redwood.dev/client/react'
import { uniq } from 'lodash'

export const Context = createContext({
    peersByAddress: {},
})

function Provider({ children }) {
    const [peersByAddress, setPeersByAddress] = useState({})
    const { nodePeers, nodeIdentities } = useRedwood()

    useEffect(() => {
        const nodeAddrs = (nodeIdentities || []).map((i) => i.address)
        const peersByAddr = {}
        for (const peer of nodePeers) {
            for (const identity of peer.identities) {
                peersByAddr[identity.address] =
                    peersByAddr[identity.address] || {}

                peersByAddr[identity.address].address = identity.address

                peersByAddr[identity.address].stateURIs =
                    peersByAddr[identity.address].stateURIs || []
                peersByAddr[identity.address].stateURIs = uniq(
                    peersByAddr[identity.address].stateURIs.concat(
                        peer.stateURIs || [],
                    ),
                )

                peersByAddr[identity.address].servers = Object.keys(
                    peersByAddr[identity.address].stateURIs
                        .filter(
                            (stateURI) => stateURI.indexOf('chat.p2p/') !== 0,
                        )
                        .reduce(
                            (total, each) => ({
                                ...total,
                                [each.split('/')[0]]: true,
                            }),
                            {},
                        ),
                )

                if (
                    peersByAddr[identity.address].lastContact &&
                    peer.lastContact
                ) {
                    peersByAddr[identity.address].lastContact = new Date(
                        max(
                            peersByAddr[identity.address].lastContact || 0,
                            peer.lastContact.getTime(),
                        ),
                    )
                } else {
                    peersByAddr[identity.address].lastContact = peer.lastContact
                }
                peersByAddr[identity.address].isSelf = nodeAddrs.includes(
                    identity.address,
                )

                peersByAddr[identity.address].transports =
                    peersByAddr[identity.address].transports || {}
                peersByAddr[identity.address].transports[peer.transport] =
                    peersByAddr[identity.address].transports[peer.transport] ||
                    []
                peersByAddr[identity.address].transports[peer.transport].push(
                    peer.dialAddr,
                )
            }
        }
        setPeersByAddress(peersByAddr)
    }, [nodePeers, nodeIdentities])

    return (
        <Context.Provider value={{ peersByAddress }}>
            {children}
        </Context.Provider>
    )
}

function max(a, b) {
    return a < b ? b : a
}

export default Provider
