import { useEffect, useState, useMemo } from 'react'
import useRedwood from './useRedwood'
import useStateTree from './useStateTree'
import useAddressBook from './useAddressBook'
import useUsers from './useUsers'

function useRoomName(server, room) {
    const stateURI = server && room ? `${server}/${room}` : null
    const roomState = useStateTree(stateURI)
    const { nodeIdentities, privateTreeMembers } = useRedwood()
    const addressBook = useAddressBook()
    const { users } = useUsers(stateURI)
    const [roomName, setRoomName] = useState(room)

    const selectedURITree = useMemo(
        () => (privateTreeMembers || {})[stateURI],
        [privateTreeMembers, stateURI],
    )

    useEffect(() => {
        if (!roomState) {
            setRoomName(room)
        } else if (roomState && roomState.name) {
            setRoomName(roomState.name)
        } else if (
            server === 'chat.p2p' &&
            privateTreeMembers &&
            privateTreeMembers[stateURI] &&
            privateTreeMembers[stateURI].length > 0
        ) {
            const nodeAddrs = (nodeIdentities || []).map(
                (identity) => identity.address,
            )
            setRoomName(
                privateTreeMembers[stateURI]
                    .map((addr) => (nodeAddrs.includes(addr) ? 'You' : addr))
                    .map((addr) =>
                        addr === 'You'
                            ? addr
                            : (addressBook || {})[addr] ||
                              ((users || {})[addr] || {}).username ||
                              addr.substr(0, 6),
                    )
                    .join(', '),
            )
        } else {
            setRoomName(room)
        }
    }, [
        server,
        room,
        roomState,
        privateTreeMembers,
        selectedURITree,
        nodeIdentities,
        users,
        addressBook,
        stateURI,
    ])

    return roomName
}

export default useRoomName
