import { useEffect, useState, useMemo, useContext } from 'react'
import { Context } from '../contexts/Braid'

function useContracts() {
    const braid = useContext(Context)
    return useMemo(() => braid, [braid])
}

export default useContracts
