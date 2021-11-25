import { useMemo, useContext } from 'react'
import { LoginStatusContext } from '../contexts/LoginStatus'

function useLoginStatus() {
    const loginStatus = useContext(LoginStatusContext)
    return useMemo(() => loginStatus, [loginStatus])
}

export default useLoginStatus
