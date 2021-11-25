import React, { createContext, useState, useEffect } from 'react'
import { useRedwood } from '@redwood.dev/client/react'
import useAddressBook from '../hooks/useAddressBook'
import useAPI from '../hooks/useAPI'

export const Context = createContext({
    servers: {
        'chat.p2p': {
            name: 'Direct messages',
            rawName: 'chat.p2p',
        },
    },
    rooms: {},
})

function Provider({ children }) {
    const [servers, setServers] = useState({})
    const [rooms, setRooms] = useState({})

    const {
        nodeIdentities,
        privateTreeMembers,
        subscribe,
        stateTrees,
        subscribedStateURIs,
    } = useRedwood()
    const addressBook = useAddressBook()
    const api = useAPI()

    useEffect(() => {
        if (!stateTrees || !privateTreeMembers) {
            return
        }

        const newServers = {}
        const newRooms = {}

        for (const stateURI of Object.keys(stateTrees)) {
            if (stateURI === 'chat.local/servers') {
                for (const server of Object.keys(
                    stateTrees['chat.local/servers'].value || {},
                )) {
                    const registry = `${server}/registry`
                    if (subscribedStateURIs.current[registry]) {
                        continue
                    }
                    subscribe(registry)
                }
                continue
            }

            const [server, room] = stateURI.split('/')

            const isDirectMessage = server === 'chat.p2p'
            let registryStateURI
            if (isDirectMessage) {
                registryStateURI = 'chat.local/dms'
            } else {
                registryStateURI = server ? `${server}/registry` : null
            }

            newServers[server] = {
                name: server,
                rawName: server,
                isDirectMessage,
                registryStateURI,
            }

            if (room === 'registry' || stateURI === 'chat.local/dms') {
                for (const lRoom of Object.keys(
                    (stateTrees[stateURI] || {}).rooms || {},
                )) {
                    const roomStateURI = `${
                        stateURI === 'chat.local/dms' ? 'chat.p2p' : server
                    }/${lRoom}`
                    if (subscribedStateURIs.current[roomStateURI]) {
                        continue
                    }
                    subscribe(roomStateURI)
                    api.subscribe(roomStateURI)
                }
            } else {
                newRooms[stateURI] = {
                    rawName: room,
                    members: privateTreeMembers[stateURI] || [],
                    isDirectMessage: stateURI.indexOf('chat.p2p/') === 0,
                }
            }
        }

        newServers['chat.p2p'] = {
            name: 'Direct messages',
            rawName: 'chat.p2p',
            isDirectMessage: true,
            registryStateURI: 'chat.local/dms',
        }

        setServers(newServers)
        setRooms(newRooms)
    }, [
        stateTrees,
        privateTreeMembers,
        nodeIdentities,
        addressBook,
        api,
        subscribe,
        subscribedStateURIs,
    ])

    return (
        <Context.Provider value={{ servers, rooms }}>
            {children}
        </Context.Provider>
    )
}

export default Provider
