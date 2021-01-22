import { useEffect, useState, useContext } from 'react'
import { Context } from '../contexts/Redwood'

function useStateTree(stateURI) {
    const { redwoodClient, subscribedStateURIs, setSubscribedStateURIs, stateTrees, updateStateTree } = useContext(Context)

    useEffect(() => {
        if (!redwoodClient || !stateURI) {
            return
        }
        (async function() {
            if (subscribedStateURIs[stateURI]) {
                return
            }
            redwoodClient.subscribe({ stateURI, keypath: '/', states: true, callback: async (err, { state, leaves }) => {
                console.log(stateURI, state)
                if (err) {
                    console.error(err)
                    return
                }
                updateStateTree(stateURI, state, leaves)
            }})
            setSubscribedStateURIs(prevURIs => ({ ...prevURIs, [stateURI]: true }))
        })()
    }, [stateURI, redwoodClient, subscribedStateURIs[stateURI], setSubscribedStateURIs])

    return stateTrees[stateURI]
}

export default useStateTree
