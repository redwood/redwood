import React, { createContext, useCallback, useState, useEffect } from 'react'
import { useRedwood } from 'redwood-p2p-client/react'
import createAPI from '../api'

export const Context = createContext(null)

function Provider({ children }) {
    const { redwoodClient, nodeIdentities } = useRedwood()
    const [api, setAPI] = useState(null)

    useEffect(() => {
        if (!redwoodClient) {
            return
        }
        let ownAddress = nodeIdentities && nodeIdentities.length > 0 ? nodeIdentities[1].address : null
        setAPI(createAPI(redwoodClient, ownAddress))
    }, [redwoodClient, nodeIdentities])

    return (
      <Context.Provider value={api}>
          {children}
      </Context.Provider>
    )
}

export default Provider