import { useEffect, useState, useMemo, useContext } from 'react'
import useNavigation from './useNavigation'
import useServerAndRoomInfo from './useServerAndRoomInfo'

function useCurrentServerAndRoom() {
    const { selectedServer, selectedStateURI } = useNavigation()
    const { servers, rooms } = useServerAndRoomInfo()
    return {
        currentServer: servers[selectedServer],
        currentRoom:   rooms[selectedStateURI],
    }
}

export default useCurrentServerAndRoom
