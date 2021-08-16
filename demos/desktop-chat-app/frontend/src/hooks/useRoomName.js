import { useEffect, useState, useMemo, useContext } from 'react'
import { useRedwood, useStateTree } from '@redwood.dev/client/react'
import useAddressBook from './useAddressBook'
import useUsers from './useUsers'

function useRoomName(server, room) {
    const stateURI = server && room ? `${server}/${room}` : null
    const roomState = useStateTree(stateURI)
    const { nodeIdentities, privateTreeMembers } = useRedwood()
    const addressBook = useAddressBook()
    const { users } = useUsers(stateURI)
    const [roomName, setRoomName] = useState(room)

    useEffect(() => {
        if (!roomState) {
            setRoomName(room)

        } else if (roomState && roomState.name) {
            setRoomName(roomState.name)

        } else if (server === 'chat.p2p' && privateTreeMembers && privateTreeMembers[stateURI] && privateTreeMembers[stateURI].length > 0) {
            let nodeAddrs = (nodeIdentities || []).map(identity => identity.address)
            setRoomName(privateTreeMembers[stateURI]
                            .map(addr => nodeAddrs.includes(addr) ? 'You' : addr)
                            .map(addr => addr === 'You' ? addr : (addressBook || {})[addr] || ((users || {})[addr] || {}).username || addr.substr(0, 6))
                            .join(', '))
        } else {
            setRoomName(room)
        }
    }, [server, room, roomState, privateTreeMembers, (privateTreeMembers || {})[stateURI], nodeIdentities, users, addressBook])

    return roomName
}

export default useRoomName
