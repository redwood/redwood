import { useCallback, useContext } from 'react'
import { Context } from '../contexts/Modals'

function useModal(modalKey) {
    const { onDismiss, onPresent, activeModalKey, activeModalProps } = useContext(Context)

    const handlePresent = useCallback((activeModalProps) => {
        onPresent(modalKey, activeModalProps)
    }, [modalKey, onPresent])

    const handleDismiss = useCallback(() => {
        onDismiss(modalKey)
    }, [modalKey, onDismiss])

    return {
        onPresent: handlePresent,
        onDismiss: handleDismiss,
        activeModalKey,
        activeModalProps,
    }
}

export default useModal
