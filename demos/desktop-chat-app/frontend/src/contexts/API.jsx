import React, { createContext, useCallback, useState, useEffect } from 'react'
import rpcFetch from '../utils/rpcFetch'
import Redwood from '../redwood.js'
import useRedwood from '../hooks/useRedwood'
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