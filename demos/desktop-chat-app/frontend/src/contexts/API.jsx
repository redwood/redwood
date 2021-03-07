import React, { createContext, useCallback, useState, useEffect } from 'react'
import rpcFetch from '../utils/rpcFetch'
import { useRedwood } from 'redwood/dist/main/react'
import createAPI from '../api'

export const Context = createContext(null)

function Provider({ children }) {
    const { redwoodClient } = useRedwood()
    const [api, setAPI] = useState(null)

    useEffect(() => {
        if (!redwoodClient) {
            return
        }
        setAPI(createAPI(redwoodClient))
    }, [redwoodClient])

    return (
      <Context.Provider value={api}>
          {children}
      </Context.Provider>
    )
}

export default Provider