import { useRef, useState, useEffect } from 'react'
import useStateTree from './useStateTree'
import useServerAndRoomInfo from './useServerAndRoomInfo'
import useAddressBook from './useAddressBook'

function useUsers(stateURI) {
    const { rooms } = useServerAndRoomInfo()
    const defaultValue = useRef({})
    const [retval, setRetval] = useState({})
    const addressBook = useAddressBook()

    const [server] = (stateURI || '').split('/')
    const { isDirectMessage } = rooms[stateURI] || {}

    let usersStateURI
    if (isDirectMessage && !!stateURI) {
        usersStateURI = stateURI
    } else if (!isDirectMessage && server && server.length > 0) {
        usersStateURI = `${server}/registry`
    }
    const usersTree = useStateTree(usersStateURI)
    const users = usersTree ? usersTree.users : defaultValue.current

    useEffect(() => {
        const newUsers = {}
        for (const key of Object.keys(users)) {
            newUsers[key] = {
                ...users[key],
                nickname: addressBook[key],
            }
        }

        setRetval({ users: newUsers, usersStateURI })
    }, [users, usersStateURI, addressBook])

    return retval
}

export default useUsers
