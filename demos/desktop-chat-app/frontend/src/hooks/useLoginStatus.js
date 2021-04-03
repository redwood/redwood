import { useEffect, useState, useMemo, useContext } from 'react'
import { Context } from '../contexts/LoginStatus'

function useLoginStatus() {
    const api = useContext(Context)
    return useMemo(() => api, [api])
}

export default useLoginStatus
