import React, { createContext, useCallback, useState, useEffect } from 'react'

export const Context = createContext({
    signup: () => {},
    login: () => {},
    logout: () => {},
    checkLogin: () => {},
    isLoggedIn: false,
    profilesFetched: false,
    profileNames: [],
})

function Provider({ apiEndpoint, children }) {
    const [isLoggedIn, setIsLoggedIn] = useState(false)
    const [profileNames, setProfileNames] = useState([])
    const [profilesFetched, setProfilesFetched] = useState(false)

    const getProfileNames = useCallback(async () => {
        try {
            const resp = await (
                await fetch(`${apiEndpoint}/api/profile-names`, {
                    method: 'GET',
                })
            ).json()
            setProfileNames(resp.profileNames || [])
            setProfilesFetched(true)
        } catch (err) {
            setProfileNames([])
        }
    }, [apiEndpoint])

    const signup = useCallback(
        async ({ profileName }) => {
            const resp = await fetch(`${apiEndpoint}/api/confirm-profile`, {
                method: 'POST',
                body: JSON.stringify({ profileName }),
            })

            if (resp.status >= 400) {
                const errorText = await resp.text()
                throw new Error(errorText)
            }
            const createdMnemonic = await resp.text()
            await getProfileNames()
            return createdMnemonic
        },
        [apiEndpoint, getProfileNames],
    )

    const login = useCallback(
        async ({ profileName, mnemonic, password }) => {
            if (isLoggedIn) {
                return
            }

            const resp = await fetch(`${apiEndpoint}/api/login`, {
                method: 'POST',
                body: JSON.stringify({ profileName, mnemonic, password }),
            })
            if (resp.status >= 400) {
                const errorText = await resp.text()
                throw new Error(errorText)
            }
            await getProfileNames()
            setIsLoggedIn(true)
        },
        [apiEndpoint, isLoggedIn, setIsLoggedIn, getProfileNames],
    )

    const logout = useCallback(async () => {
        if (!isLoggedIn) {
            return
        }

        const resp = await fetch(`${apiEndpoint}/api/logout`, {
            method: 'POST',
        })
        if (resp.status >= 400) {
            const errorText = await resp.text()
            throw new Error(errorText)
        }
        setProfilesFetched(false)
        await getProfileNames()
        setIsLoggedIn(false)
    }, [apiEndpoint, isLoggedIn, setIsLoggedIn, getProfileNames])

    const checkLogin = useCallback(async () => {
        try {
            const resp = await fetch(`${apiEndpoint}/api/check-login`, {
                method: 'POST',
            })
            const jsonResp = await resp.text()
            setIsLoggedIn(jsonResp === 'true')
            await getProfileNames()
            return true
        } catch (err) {
            return err
        }
    }, [apiEndpoint, getProfileNames])

    useEffect(() => {
        checkLogin()
    }, [apiEndpoint, checkLogin])

    return (
        <Context.Provider
            value={{
                signup,
                login,
                logout,
                checkLogin,
                isLoggedIn,
                profileNames,
                profilesFetched,
            }}
        >
            {children}
        </Context.Provider>
    )
}

export default Provider
