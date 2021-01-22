import { useCallback, useContext } from 'react'
import { Context } from '../contexts/Modals'

function useModal(modalKey) {
    const { onDismiss, onPresent, activeModalKey } = useContext(Context)

    const handlePresent = useCallback(() => {
        onPresent(modalKey)
    }, [modalKey, onPresent])

    const handleDismiss = useCallback(() => {
        onDismiss(modalKey)
    }, [modalKey, onDismiss])

    return {
        onPresent: handlePresent,
        onDismiss: handleDismiss,
        activeModalKey,
    }
}

export default useModal
