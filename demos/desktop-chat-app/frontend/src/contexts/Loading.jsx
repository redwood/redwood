import { createContext, useState, useCallback } from 'react'

export const LoadingContext = createContext({
    loadingTrees: {},
    setLoadingTree: () => {},
})

function LoadingProvider({ children }) {
    const [loadingTrees, hookSetLoadingTree] = useState({})

    const setLoadingTree = useCallback(
        (key, value) => {
            hookSetLoadingTree({
                ...loadingTrees,
                [key]: value,
            })
        },
        [loadingTrees, hookSetLoadingTree],
    )

    return (
        <LoadingContext.Provider value={{ setLoadingTree, loadingTrees }}>
            {children}
        </LoadingContext.Provider>
    )
}

export default LoadingProvider
