import { useMemo, useContext } from 'react'
import { ServerAndRoomInfoContext } from '../contexts/ServerAndRoomInfo'

function useServerAndRoomInfo() {
    const info = useContext(ServerAndRoomInfoContext)
    return useMemo(() => info, [info])
}

export default useServerAndRoomInfo
