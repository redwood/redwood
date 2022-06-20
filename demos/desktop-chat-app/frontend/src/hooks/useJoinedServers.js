import { useMemo } from 'react'
import { useStateTree } from '@redwood.dev/react'

function useJoinedServers() {
    const [joinedServersTree] = useStateTree('chat.local/servers')
    return useMemo(() => Object.keys((joinedServersTree || {}).value || {}).filter(x => !!x), [joinedServersTree])
}

export default useJoinedServers




