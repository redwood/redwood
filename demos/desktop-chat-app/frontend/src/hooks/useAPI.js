import { useMemo, useContext } from 'react'
import { APIContext } from '../contexts/API'

function useAPI() {
    const api = useContext(APIContext)
    return useMemo(() => api, [api])
}

export default useAPI
