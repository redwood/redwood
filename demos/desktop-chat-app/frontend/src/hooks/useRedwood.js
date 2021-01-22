import { useEffect, useState, useMemo, useContext } from 'react'
import { Context } from '../contexts/Redwood'

function useRedwood() {
    const braid = useContext(Context)
    return useMemo(() => braid, [braid])
}

export default useRedwood
