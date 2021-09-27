import { useMemo, useContext } from 'react'
import { Context } from '../contexts/LoginStatus'

function useLoginStatus() {
    const loginStatus = useContext(Context)
    return useMemo(() => loginStatus, [loginStatus])
}

export default useLoginStatus
