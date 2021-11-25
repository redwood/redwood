import { useMemo, useContext } from 'react'
import { Context } from '../contexts/Navigation'
import useServerAndRoomInfo from './useServerAndRoomInfo'

function useNavigation() {
    const navigation = useContext(Context)
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
