import { useCallback, useContext } from 'react'
import { ModalsContext } from '../contexts/Modals'

function useModal(modalKey) {
    const { onDismiss, onPresent, activeModalKey, activeModalProps } =
        useContext(ModalsContext)

    const handlePresent = useCallback(
        (modalProps) => {
            onPresent(modalKey, modalProps)
        },
        [modalKey, onPresent],
    )

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
