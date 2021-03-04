import { useEffect, useState, useMemo, useContext } from 'react'
import { Context } from './RedwoodProvider'

function useRedwood() {
    const redwood = useContext(Context)
    return useMemo(() => redwood, [redwood])
}

export default useRedwood
