import { useMemo } from 'react'
import { useStateTree } from 'redwood/dist/main/react'

function useJoinedServers() {
    const joinedServersTree = useStateTree('chat.local/servers')
    const joinedServers = Object.keys((joinedServersTree || {}).value || {}).filter(x => !!x)
    return useMemo(() => joinedServers, [joinedServers])
}

export default useJoinedServers




