import React, { createContext, useState, useEffect } from 'react'
import { useRedwood } from '@redwood.dev/client/react'
import createAPI from '../api'

export const Context = createContext(null)

function Provider({ children }) {
    const { redwoodClient, nodeIdentities } = useRedwood()
    const [api, setAPI] = useState(null)

    useEffect(() => {
        if (!redwoodClient) {
            return
        }
        const ownAddress =
            nodeIdentities && nodeIdentities.length > 0
                ? nodeIdentities[0].address
                : null
        setAPI(createAPI(redwoodClient, ownAddress))
    }, [redwoodClient, nodeIdentities])

    return <Context.Provider value={api}>{children}</Context.Provider>
}

export default Provider
