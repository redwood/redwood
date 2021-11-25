import { useMemo, useContext } from 'react'
import { Context } from '../contexts/API'

function useAPI() {
    const api = useContext(Context)
    return useMemo(() => api, [api])
}

export default useAPI
