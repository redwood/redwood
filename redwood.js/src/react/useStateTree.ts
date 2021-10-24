import {
    useEffect,
    useState,
    useCallback,
    useContext,
    useDebugValue,
} from 'react'
import useRedwood from './useRedwood'

function useStateTree(stateURI: string | null | undefined, keypath?: string) {
    const {
        redwoodClient,
        httpHost,
        useWebsocket,
        subscribe,
        subscribedStateURIs = {},
        stateTrees,
        leaves,
        privateTreeMembers,
        updatePrivateTreeMembers,
        updateStateTree,
        getStateTree,
    } = useRedwood()

    const keypath_ = (keypath || '').length === 0 ? '/' : keypath

    useEffect(() => {
        (async function () {
            if (!redwoodClient || !stateURI || !updatePrivateTreeMembers) {
                return
            }
            // @@TODO: just read from the `.Members` keypath
            if (redwoodClient.rpc) {
                const { rpc } = redwoodClient
                getStateTree('privateTreeMembers', (currPTMembers: any) => {
                    // If stateURI do not exist on privateTreeMembers fetch members and add to state
                    if (!currPTMembers.hasOwnProperty(stateURI)) {
                        rpc.privateTreeMembers(stateURI).then((members) => {
                            updatePrivateTreeMembers(stateURI, members)
                        })
                    }
                })
            }
        })()
    }, [redwoodClient, stateURI])

    useEffect(() => {
        if (!stateURI) {
            return
        }

        subscribe(stateURI, (err, data) => {
            console.log(err, data)
        })
    }, [subscribe, stateURI, leaves, stateTrees])

    return stateURI ? stateTrees[stateURI] : null
}

export default useStateTree
