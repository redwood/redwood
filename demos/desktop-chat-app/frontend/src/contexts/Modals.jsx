import React, { createContext, useCallback, useState } from 'react'
import styled from 'styled-components'

export const Context = createContext({
    activeModalKey: null,
    onPresent: () => {},
    onDismiss: () => {},
})

function Modals({ children }) {
    const [activeModalKey, setActiveModalKey] = useState()

    const handlePresent = useCallback((key) => {
        setActiveModalKey(key)
    }, [setActiveModalKey])

    const handleDismiss = useCallback((key) => {
        if (activeModalKey === key) {
            setActiveModalKey(undefined)
        }
    }, [activeModalKey])

    return (
        <Context.Provider value={{
            activeModalKey,
            onPresent: handlePresent,
            onDismiss: handleDismiss,
        }}>
            {children}
            <div id="modal-root" />
        </Context.Provider>
    )
}

export default Modals