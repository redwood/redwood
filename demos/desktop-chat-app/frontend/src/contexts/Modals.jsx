import React, { createContext, useCallback, useState } from 'react'
import styled from 'styled-components'

export const Context = createContext({
    activeModalKey: null,
    activeModalProps: {},
    onPresent: () => {},
    onDismiss: () => {},
})

function Modals({ children }) {
    const [activeModalKey, setActiveModalKey] = useState()
    const [activeModalProps, setActiveModalProps] = useState({})

    const handlePresent = useCallback((key, activeModalProps) => {
        setActiveModalProps(activeModalProps || {})
        setActiveModalKey(key)
    }, [setActiveModalKey, setActiveModalProps])

    const handleDismiss = useCallback((key) => {
        if (activeModalKey === key) {
            setActiveModalKey(undefined)
        }
        setActiveModalProps({})
    }, [activeModalKey, setActiveModalKey, setActiveModalProps])

    return (
        <Context.Provider value={{
            activeModalKey,
            activeModalProps,
            onPresent: handlePresent,
            onDismiss: handleDismiss,
        }}>
            {children}
            <div id="modal-root" />
        </Context.Provider>
    )
}

export default Modals