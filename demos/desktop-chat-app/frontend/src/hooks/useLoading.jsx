import { useContext, useMemo } from 'react'
import { LoadingContext } from '../contexts/Loading'

function useLoading() {
    const loadingContext = useContext(LoadingContext)

    return useMemo(() => loadingContext, [loadingContext])
}

export default useLoading
