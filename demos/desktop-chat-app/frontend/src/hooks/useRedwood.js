import { useEffect, useState, useMemo, useContext } from 'react'
import { Context } from '../contexts/Redwood'

function useRedwood() {
    const redwood = useContext(Context)
    return useMemo(() => redwood, [redwood])
}

export default useRedwood
