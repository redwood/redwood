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

        let conn = new WebSocket(`ws://localhost:54231/ws?state_uri=${encodeURIComponent(stateURI)}`)
        conn.onclose = function (evt) {}
        conn.onmessage = function (evt) {
            let messages = evt.data.split('\n').filter(x => x.trim().length > 0)
            for (let msg of messages) {
                try {
                    let { state, leaves } = JSON.parse(msg)
                    updateStateTree(stateURI, state, leaves)
                } catch (err) {
                    console.error(err, 'message', msg)
                }
            }
        }

        // redwoodClient.rpc.subscribe({ stateURI, keypath: '/', states: true, txs: true })
        // const unsubscribePromise = redwoodClient.subscribe({ stateURI, keypath: '/', states: true, callback: async (err, next) => {
        //     if (err) {
        //         console.error(err)
        //         return
        //     }
        //     let { state, leaves } = next
        //     updateStateTree(stateURI, state, leaves)
        // }})

        subscribedStateURIs.current[stateURI] = true

        return () => {
            conn.close()
            // const unsubscribe = await unsubscribePromise
            // unsubscribe()
            subscribedStateURIs.current[stateURI] = false
        }

    }, [redwoodClient, stateURI])

    return stateTrees[stateURI]
}

export default useStateTree
