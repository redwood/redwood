import { useMemo, useContext } from 'react'
import { Context } from '../contexts/ServerAndRoomInfo'

function useServerAndRoomInfo() {
    const info = useContext(Context)
    return useMemo(() => info, [info])
}

export default useServerAndRoomInfo
