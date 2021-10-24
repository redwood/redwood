import { useMemo, useContext } from 'react'
import { RedwoodContext } from './RedwoodProvider'

function useRedwood() {
    const redwood = useContext(RedwoodContext)
    return useMemo(() => redwood, [redwood])
}

export default useRedwood
