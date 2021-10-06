import { createContext, useCallback, useState } from 'react'

export const ModalsContext = createContext({
    activeModalKey: null,
    activeModalProps: {},
    onPresent: () => {},
    onDismiss: () => {},
})

function ModalsProvider({ children }) {
    const [activeModalKey, setActiveModalKey] = useState()
    const [activeModalProps, setActiveModalProps] = useState({})

    const handlePresent = useCallback(
        (key, modalProps) => {
            setActiveModalProps(modalProps || {})
            setActiveModalKey(key)
        },
        [setActiveModalKey, setActiveModalProps],
    )

    const handleDismiss = useCallback(
        (key) => {
            if (activeModalKey === key) {
                setActiveModalKey(undefined)
            }
            setActiveModalProps({})
        },
        [activeModalKey, setActiveModalKey, setActiveModalProps],
    )

    return (
        <ModalsContext.Provider
            value={{
                activeModalKey,
                activeModalProps,
                onPresent: handlePresent,
                onDismiss: handleDismiss,
            }}
        >
            {children}
            <div id="modal-root" />
        </ModalsContext.Provider>
    )
}

export default ModalsProvider
