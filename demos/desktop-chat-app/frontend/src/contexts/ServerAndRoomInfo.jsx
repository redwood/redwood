import {
    createContext,
    useCallback,
    useReducer,
    useMemo,
    useEffect,
} from 'react'
import { usePrevious } from 'react-use'
import deepCompare from 'fast-deep-equal/es6/react'
import useRedwood from '../hooks/useRedwood'
import useAPI from '../hooks/useAPI'
import useAddressBook from '../hooks/useAddressBook'
import useLoading from '../hooks/useLoading'

export const ServerAndRoomInfoContext = createContext({
    servers: {
        'chat.p2p': {
            name: 'Direct messages',
            rawName: 'chat.p2p',
        },
    },
    rooms: {},
})

function reducer(state, action) {
    switch (action.type) {
        case 'updateServersRooms': {
            const newServersRooms = {
                servers: {
                    ...state.servers,
                    ...action.newServers,
                },
                rooms: {
                    ...state.rooms,
                    ...action.newRooms,
                },
            }

            if (!deepCompare(newServersRooms, state)) {
                return newServersRooms
            }

            return state
        }
        default: {
            return state
        }
    }
}

function ServerAndRoomInfoProvider({ children }) {
    const [state, dispatch] = useReducer(reducer, {
        servers: {
            'chat.p2p': {
                name: 'Direct messages',
                rawName: 'chat.p2p',
            },
        },
        rooms: {},
    })

    const { privateTreeMembers, subscribe, stateTrees, subscribedStateURIs } =
        useRedwood()
    const { loadingTrees, setLoadingTree } = useLoading()
    const api = useAPI()
    const addressBook = useAddressBook()
    const prevStateTrees = usePrevious(stateTrees)

    const stateURIs = useMemo(() => Object.keys(stateTrees), [stateTrees])

    const getServerAndRoomInfo = useCallback(() => {
        const newServers = {}
        const newRooms = {}

        stateURIs.forEach((stateURI) => {
            if (stateURI === 'chat.local/servers') {
                const stateServers = Object.keys(
                    stateTrees['chat.local/servers'].value || {},
                )

                stateServers.forEach((server) => {
                    const registry = `${server}/registry`

                    if (!subscribedStateURIs[registry]) {
                        if (!loadingTrees[registry]) {
                            setLoadingTree(stateURI, true)
                            subscribe(registry, (err, data) => {
                                setLoadingTree(stateURI, false)
                            })
                        }
                    }
                })
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
                const lRooms = Object.keys(
                    (stateTrees[stateURI] || {}).rooms || {},
                )

                lRooms.forEach((lRoom) => {
                    const roomStateURI = `${
                        stateURI === 'chat.local/dms' ? 'chat.p2p' : server
                    }/${lRoom}`

                    if (!subscribedStateURIs[roomStateURI]) {
                        if (!loadingTrees[roomStateURI]) {
                            setLoadingTree(roomStateURI, true)

                            subscribe(roomStateURI, (err, data) => {
                                setLoadingTree(stateURI, false)
                            })
                            api.subscribe(roomStateURI)
                        }
                    }
                })
            } else {
                newRooms[stateURI] = {
                    rawName: room,
                    members: privateTreeMembers[stateURI] || [],
                    isDirectMessage: stateURI.indexOf('chat.p2p/') === 0,
                }
            }
        })

        newServers['chat.p2p'] = {
            name: 'Direct messages',
            rawName: 'chat.p2p',
            isDirectMessage: true,
            registryStateURI: 'chat.local/dms',
        }

        dispatch({
            type: 'updateServersRooms',
            newServers,
            newRooms,
        })
    }, [
        stateTrees,
        api,
        subscribe,
        subscribedStateURIs,
        privateTreeMembers,
        dispatch,
        setLoadingTree,
        loadingTrees,
    ])

    useEffect(() => {
        if (!deepCompare(prevStateTrees, stateTrees)) {
            getServerAndRoomInfo()
        }
    }, [getServerAndRoomInfo, privateTreeMembers, addressBook])

    return (
        <ServerAndRoomInfoContext.Provider
            value={{ servers: state.servers, rooms: state.rooms }}
        >
            {children}
        </ServerAndRoomInfoContext.Provider>
    )
}

export default ServerAndRoomInfoProvider
