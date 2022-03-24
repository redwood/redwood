import { useEffect, useState, useMemo, useContext } from 'react'
import { useRedwood, useStateTree } from '@redwood.dev/react'
import { get } from 'lodash'
import useAddressBook from './useAddressBook'
import useUsers from './useUsers'

function useRoomName(server, room) {
    const stateURI = server && room ? `${server}/${room}` : null
    const [roomState] = useStateTree(stateURI)
    const { nodeIdentities } = useRedwood()
    const addressBook = useAddressBook()
    const { users } = useUsers(stateURI)
    const [roomName, setRoomName] = useState(room)

    useEffect(() => {
        if (!roomState) {
            setRoomName(room)

        } else if (roomState && roomState.name) {
            setRoomName(roomState.name)

        } else if (server === 'chat.p2p') {
            let members = Object.keys(get(roomState, ['Members']) || {})

            let nodeAddrs = (nodeIdentities || []).map(identity => identity.address)
            setRoomName(members
                            .map(addr => nodeAddrs.includes(addr) ? 'You' : addr)
                            .map(addr => addr === 'You' ? addr : (get(addressBook, addr) || get(users, addr) || {}).username || addr.substr(0, 6))
                            .join(', '))
        } else {
            setRoomName(room)
        }
    }, [server, room, roomState, nodeIdentities, users, addressBook])

    return roomName
}

export default useRoomName
