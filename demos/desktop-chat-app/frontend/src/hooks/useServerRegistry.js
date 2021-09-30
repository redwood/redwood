import { useMemo } from 'react'
import { useStateTree } from '../components/redwood.js/dist/main/react'
import useServerAndRoomInfo from './useServerAndRoomInfo'

function useServerRegistry(serverName) {
    const { servers } = useServerAndRoomInfo()
    const { registryStateURI } = servers[serverName] || {}
    const registry = useStateTree(registryStateURI)
    return useMemo(
        () => ({ registry, registryStateURI }),
        [registry, registryStateURI],
    )
}

export default useServerRegistry
