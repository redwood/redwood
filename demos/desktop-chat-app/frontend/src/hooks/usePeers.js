import { useMemo, useContext } from 'react'
import { Context } from '../contexts/Peers'

function usePeers() {
    const peers = useContext(Context)
    return useMemo(() => peers, [peers])
}

export default usePeers
