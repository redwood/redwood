import React, { createContext, useCallback, useState, useEffect } from 'react'
import { useRedwood } from 'redwood/dist/main/react'

export const Context = createContext({
    signup: () => {},
    login: () => {},
    logout: () => {},
    checkLogin: () => {},
    isLoggedIn: false,
    profileNames: [],
})

function Provider({ apiEndpoint, children }) {
    let [isLoggedIn, setIsLoggedIn] = useState(false)
    let [profileNames, setProfileNames] = useState([])

    let signup = useCallback(async ({ profileName }) => {
        let resp = await fetch(`${apiEndpoint}/api/confirm-profile`, {
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
    }, [apiEndpoint])

    let login = useCallback(async ({ profileName, mnemonic, password }) => {
        if (isLoggedIn) {
            return
        }

        let resp = await fetch(`${apiEndpoint}/api/login`, {
            method: 'POST',
            body: JSON.stringify({ profileName, mnemonic, password }),
        })
        if (resp.status >= 400) {
            const errorText = await resp.text()
            throw new Error(errorText)
        }
        await getProfileNames()
        setIsLoggedIn(true)

    }, [apiEndpoint, isLoggedIn, setIsLoggedIn])

    let logout = useCallback(async () => {
        if (!isLoggedIn) {
            return
        }

        let resp = await fetch(`${apiEndpoint}/api/logout`, { method: 'POST' })
        if (resp.status >= 400) {
            const errorText = await resp.text()
            throw new Error(errorText)
        }
        await getProfileNames()
        setIsLoggedIn(false)
    }, [apiEndpoint, isLoggedIn, setIsLoggedIn])


    let checkLogin = useCallback(async () => {
        try {
            let resp = await fetch(`${apiEndpoint}/api/check-login`, { method: 'POST' })
            let jsonResp = await resp.text()
            setIsLoggedIn(jsonResp === 'true')
            await getProfileNames()
        } catch (err) {
            console.error(err)
        }
    }, [apiEndpoint])

    async function getProfileNames() {
        try {
            let resp = await (await fetch(`${apiEndpoint}/api/profile-names`, { method: 'GET' })).json()
            setProfileNames(resp.profileNames || [])
        } catch (err) {
            setProfileNames([])
        }
    }

    useEffect(() => {
        checkLogin()
    }, [apiEndpoint])

    return (
      <Context.Provider value={{ signup, login, logout, checkLogin, isLoggedIn, profileNames }}>
          {children}
      </Context.Provider>
    )
}

export default Provider