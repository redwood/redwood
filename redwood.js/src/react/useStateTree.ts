import { useEffect, useState, useCallback, useContext, useDebugValue } from 'react'
import useRedwood from './useRedwood'

function useStateTree(stateURI: string | null | undefined, keypath?: string) {
    const {
        redwoodClient,
        httpHost,
        useWebsocket,
        subscribe,
        subscribedStateURIs,
        stateTrees,
        privateTreeMembers,
        updatePrivateTreeMembers,
        updateStateTree,
    } = useRedwood()

    const keypath_ = (keypath || '').length === 0 ? '/' : keypath

    useDebugValue({
        redwoodClient,
        httpHost,
        useWebsocket,
        subscribedStateURIs,
        stateTrees,
        privateTreeMembers,
        updatePrivateTreeMembers,
        updateStateTree,
    })

    useEffect(() => {
        ;(async function() {
            if (!redwoodClient || !stateURI || !updatePrivateTreeMembers) {
                return
            }
            // @@TODO: just read from the `.Members` keypath
            if (!!redwoodClient.rpc) {
                const members = await redwoodClient.rpc.privateTreeMembers(stateURI)
                updatePrivateTreeMembers(stateURI, members)
            }
        })()
    }, [redwoodClient, stateURI, updatePrivateTreeMembers])

    useEffect(() => {
        if (!stateURI) {
            return
        }
        const unsubscribePromise = subscribe(stateURI)

        return () => {
            (async function() {
                const unsubscribe = await unsubscribePromise
                unsubscribe()
            })()
        }

    }, [subscribe, stateURI])

    return !!stateURI ? stateTrees[stateURI] : null
}

export default useStateTree
