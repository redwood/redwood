import { useEffect, useState, useMemo, useContext } from 'react'
import { Context } from '../contexts/Navigation'

function useNavigation() {
    const navigation = useContext(Context)
    return useMemo(() => navigation, [navigation])
}

export default useNavigation
