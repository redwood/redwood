import { useEffect, useState, useCallback, useContext, useDebugValue } from 'react'
import useRedwood from './useRedwood'

function useStateTree(stateURI) {
    const { redwoodClient, subscribedStateURIs, stateTrees, updateStateTree } = useRedwood()

    useDebugValue({
        redwoodClient, stateURI, subscribedStateURIs,
    })

    useEffect(() => {
        if (!redwoodClient || !stateURI) {
            return
        } else if (subscribedStateURIs.current[stateURI]) {
            return
        }
        redwoodClient.rpc.subscribe({ stateURI, keypath: '/', states: true, txs: true })
        const unsubscribePromise = redwoodClient.subscribe({ stateURI, keypath: '/', states: true, callback: async (err, next) => {
            if (err) {
                console.error(err)
                return
            }
            let { state, leaves } = next
            updateStateTree(stateURI, state, leaves)
        }})

        subscribedStateURIs.current[stateURI] = true

        return async () => {
            const unsubscribe = await unsubscribePromise
            unsubscribe()
            subscribedStateURIs.current[stateURI] = false
        }

    }, [redwoodClient, stateURI])

    return stateTrees[stateURI]
}

export default useStateTree
