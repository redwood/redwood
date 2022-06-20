import { useRef, useState, useMemo, useContext, useEffect } from 'react'
import { useStateTree } from '@redwood.dev/react'
import { get } from 'lodash'
import useNavigation from './useNavigation'
import useServerAndRoomInfo from './useServerAndRoomInfo'
import useAddressBook from './useAddressBook'

function useUsers(stateURI) {
    const { servers, rooms } = useServerAndRoomInfo()
    const defaultValue = useRef({})
    const [retval, setRetval] = useState({})
    const addressBook = useAddressBook()

    const [server, room] = (stateURI || '').split('/')
    const isDirectMessage = get(rooms, [stateURI, 'isDirectMessage'], false)

    let usersStateURI
    if (isDirectMessage && !!stateURI) {
        usersStateURI = stateURI
    } else if (!isDirectMessage && server && server.length > 0) {
        usersStateURI = `${server}/registry`
    }
    const [usersTree] = useStateTree(usersStateURI)
    const users = (usersTree || {}).users || defaultValue.current

    useEffect(() => {
        let newUsers = {}
        for (let key of Object.keys(users)) {
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


