import { useMemo, useContext } from 'react'
import { PeersContext } from '../contexts/Peers'

function usePeers() {
    const peers = useContext(PeersContext)
    return useMemo(() => peers, [peers])
}

export default usePeers
