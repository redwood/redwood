import { useEffect, useState, useMemo, useContext } from 'react'
import { Context } from '../contexts/Redwood'
import useRedwood from './useRedwood'
import useStateTree from './useStateTree'
import * as api from '../api'

function useKnownServers() {
    const { nodeAddress, redwoodClient } = useRedwood()
    const knownServersTree = useStateTree('chat.local/servers')

    useEffect(() => {
        if (!redwoodClient || !nodeAddress) {
            return
        }
        (async function() {
            try {
                let servers = await redwoodClient.get({ stateURI: 'chat.local/servers', keypath: '/' })
            } catch (err) {
                if (err.statusCode === 404) {
                    await api.initializeLocalServerRegistry(nodeAddress)
                }
            }
        })()
    }, [nodeAddress, redwoodClient, api.initializeLocalServerRegistry])

    console.log('knownServersTree', knownServersTree)
    return (knownServersTree || {}).value || []
}

export default useKnownServers






