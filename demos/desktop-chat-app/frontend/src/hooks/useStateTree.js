import { useEffect, useState, useContext, useMemo } from 'react'
import { useDeepCompareEffect } from 'react-use'
import deepCompare from 'fast-deep-equal/es6/react'

import useRedwood from './useRedwood'
import useLoading from './useLoading'

function useStateTree(stateURI) {
    const {
        redwoodClient,
        subscribe,
        stateTrees,
        updatePrivateTreeMembers,
        getStateTree,
        privateTreeMembers,
    } = useRedwood()
    const { loadingTrees, setLoadingTree } = useLoading()
    const [stateTree, setStateTree] = useState(null)

    const isLoadingTree = useMemo(
        () => loadingTrees[stateURI],
        [stateURI, loadingTrees],
    )

    useEffect(() => {
        ;(async function checkNewPrivateTreeMembers() {
            if (!redwoodClient || !stateURI || !updatePrivateTreeMembers) {
                return
            }
            // @@TODO: just read from the `.Members` keypath
            const { rpc } = redwoodClient
            if (rpc) {
                if (!privateTreeMembers[stateURI]) {
                    rpc.privateTreeMembers(stateURI).then((members) => {
                        updatePrivateTreeMembers(stateURI, members)
                    })
                }
                // getStateTree('privateTreeMembers', (currPTMembers) => {
                //     // If stateURI do not exist on privateTreeMembers fetch members and add to state
                //     const hasStateURIKey =
                //         Object.prototype.hasOwnProperty.call(
                //             currPTMembers,
                //             stateURI,
                //         )
                //     if (!hasStateURIKey) {
                //         rpc.privateTreeMembers(stateURI).then((members) => {
                //             updatePrivateTreeMembers(stateURI, members)
                //         })
                //     }
                // })
            }
        })()
    }, [redwoodClient, stateURI, privateTreeMembers])

    useDeepCompareEffect(() => {
        if (stateTrees[stateURI]) {
            const isEqual = deepCompare(stateTree, stateTrees[stateURI])
            if (!isEqual) {
                setStateTree({ ...stateTrees[stateURI], isLoadingTree })
            }
        } else {
            setStateTree(false)
        }
    }, [stateTrees, stateURI, isLoadingTree, loadingTrees, privateTreeMembers])

    useEffect(() => {
        if (!stateURI || !redwoodClient) {
            return
        }

        if (isLoadingTree) {
            return
        }

        if (!stateTree) {
            setLoadingTree(stateURI, true)
            subscribe(stateURI, (err, data) => {
                setLoadingTree(stateURI, false)
            })
        }
    }, [
        subscribe,
        stateURI,
        setLoadingTree,
        stateTree,
        redwoodClient,
        isLoadingTree,
    ])

    return stateTree
}

export default useStateTree
