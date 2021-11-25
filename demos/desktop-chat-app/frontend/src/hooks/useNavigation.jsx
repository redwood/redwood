import { useMemo, useContext } from 'react'
import { NavigationContext } from '../contexts/Navigation'
import useServerAndRoomInfo from './useServerAndRoomInfo'

function useNavigation() {
    const navigation = useContext(NavigationContext)
    const { servers } = useServerAndRoomInfo()

    return useMemo(
        () => ({
            ...navigation,
            registryStateURI: (servers[navigation.selectedServer] || {})
                .registryStateURI,
            isDirectMessage: (servers[navigation.selectedServer] || {})
                .isDirectMessage,
        }),
        [navigation, servers],
    )
}

export default useNavigation
