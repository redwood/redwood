import { createContext, useState, useEffect } from 'react'
import useRedwood from '../hooks/useRedwood'
import createAPI from '../api'

export const APIContext = createContext(null)

function APIProvider({ children }) {
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

    return <APIContext.Provider value={api}>{children}</APIContext.Provider>
}

export default APIProvider
