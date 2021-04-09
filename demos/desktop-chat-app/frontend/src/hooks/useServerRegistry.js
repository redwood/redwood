import { useMemo } from 'react'
import { useStateTree } from 'redwood-p2p-client/react'
import useServerAndRoomInfo from './useServerAndRoomInfo'

function useServerRegistry(serverName) {
    const { servers } = useServerAndRoomInfo()
    const registryStateURI = (servers[serverName] || {}).registryStateURI
    const registry = useStateTree(registryStateURI)
    return useMemo(() => ({ registry, registryStateURI }), [registry, registryStateURI])
}

export default useServerRegistry


