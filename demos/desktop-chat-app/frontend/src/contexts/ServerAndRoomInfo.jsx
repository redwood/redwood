import React, { createContext, useCallback, useState, useEffect, useDebugValue } from 'react'
import { useRedwood, useStateTree } from 'redwood/dist/main/react'
import useAddressBook from '../hooks/useAddressBook'
import useNavigation from '../hooks/useNavigation'
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

    const { nodeIdentities, privateTreeMembers, subscribe, stateTrees, subscribedStateURIs } = useRedwood()
    const addressBook = useAddressBook()
    const api = useAPI()

    useEffect(() => {
        if (!stateTrees || !privateTreeMembers) {
            return
        }

        let newServers = {}
        let newRooms = {}

        for (let stateURI of Object.keys(stateTrees)) {
            let [ server, room ] = stateURI.split('/')

            // if (server === 'chat.local') {
            //     continue
            // }

            let isDirectMessage = server === 'chat.p2p'
            let registryStateURI
            if (isDirectMessage) {
                registryStateURI = 'chat.local/dms'
            } else {
                registryStateURI = !!server ? `${server}/registry` : null
            }

            newServers[server] = {
                name:    server,
                rawName: server,
                isDirectMessage,
                registryStateURI,
            }

            if (room === 'registry' || stateURI === 'chat.local/dms') {
                for (let room of Object.keys(stateTrees[stateURI].rooms || {})) {
                    let roomStateURI = `${stateURI === 'chat.local/dms' ? 'chat.p2p' : server}/${room}`
                    if (subscribedStateURIs[roomStateURI]) {
                        continue
                    }
                    console.log('xyzzy', { stateURI, registryStateURI, roomStateURI })
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
    }, [stateTrees, privateTreeMembers, nodeIdentities, addressBook])

    return (
      <Context.Provider value={{ servers, rooms }}>
          {children}
      </Context.Provider>
    )
}

export default Provider