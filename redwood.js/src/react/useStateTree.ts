import { useEffect, useState, useCallback, useContext, useDebugValue } from 'react'
import useRedwood from './useRedwood'

function useStateTree(stateURI: string | null | undefined, keypath?: string) {
    const { redwoodClient, httpHost, useWebsocket, subscribedStateURIs, stateTrees, updateStateTree } = useRedwood()

    let keypath_ = (keypath || '').length === 0 ? '/' : keypath

    useDebugValue({ redwoodClient, stateURI, keypath: keypath_, subscribedStateURIs, stateTrees })

    useEffect(() => {
        if (!redwoodClient || !stateURI) {
            return
        } else if (subscribedStateURIs.current[stateURI]) {
            return
        }

        const unsubscribePromise = redwoodClient.subscribe({
            stateURI,
            keypath: '/',
            states: true,
            useWebsocket,
            callback: async (err, next) => {
                if (err) {
                    console.error(err)
                    return
                }
                let { state, leaves } = next
                updateStateTree(stateURI, state, leaves)
            },
        })

        subscribedStateURIs.current[stateURI] = true

        return () => {
            (async function() {
                const unsubscribe = await unsubscribePromise
                unsubscribe()
                subscribedStateURIs.current[stateURI] = false
            })()
        }

    }, [redwoodClient, stateURI])

    return !!stateURI ? stateTrees[stateURI] : null
}

export default useStateTree
