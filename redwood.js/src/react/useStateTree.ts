import { useEffect, useState, useCallback, useContext, useDebugValue } from 'react'
import useRedwood from './useRedwood'

function stateURIIsNonNull(stateURI: string | null | undefined): stateURI is string {
    return !!stateURI
}

function useStateTree(stateURI: string | null | undefined, keypath?: string) {
    const { redwoodClient, httpHost, subscribedStateURIs, stateTrees, updateStateTree } = useRedwood()

    let keypath_ = (keypath || '').length === 0 ? '/' : keypath

    useDebugValue({ redwoodClient, stateURI, subscribedStateURIs })

    useEffect(() => {
        if (!redwoodClient || !stateURI) {
            return
        } else if (subscribedStateURIs.current[stateURI]) {
            return
        }

        if (httpHost.substr(0, 5) === 'http:') {
            let conn = new WebSocket(`ws://localhost:54231/ws?state_uri=${encodeURIComponent(stateURI)}`)
            conn.onclose = function (evt) {}
            conn.onmessage = function (evt) {
                let messages = (evt.data as string).split('\n').filter(x => x.trim().length > 0)
                for (let msg of messages) {
                    try {
                        let { state, leaves } = JSON.parse(msg)
                        updateStateTree(stateURI, state, leaves)
                    } catch (err) {
                        console.error(err, 'message', msg)
                    }
                }
            }
            subscribedStateURIs.current[stateURI] = true
            return () => {
                conn.close()
                subscribedStateURIs.current[stateURI] = false
            }
        } else {
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
        }

    }, [redwoodClient, stateURI])

    return !!stateURI ? stateTrees[stateURI] : null
}

export default useStateTree
