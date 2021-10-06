import { createContext, useCallback, useEffect, useReducer } from 'react'
import { createSlice } from '@reduxjs/toolkit'

const initialLoginStatusState = {
    isLoggedIn: false,
    profilesFetched: false,
    profileNames: [],
    checkedLogin: false,
    connectionError: null,
    checkingLogin: true,
}

const loginStatusInitialContext = {
    signup: () => {},
    login: () => {},
    logout: () => {},
    checkLogin: () => {},
    ...initialLoginStatusState,
}

export const LoginStatusContext = createContext(loginStatusInitialContext)

const updateIsLoggedInR = (state, action) => {
    const { isLoggedIn } = action.payload

    if (state.isLoggedIn !== isLoggedIn) {
        state.isLoggedIn = isLoggedIn
        return state
    }
}

const updateProfilesFetchedR = (state, action) => {
    const { profilesFetched } = action.payload

    if (state.profilesFetched !== profilesFetched) {
        state.profilesFetched = profilesFetched
        return state
    }
}

const updateProfileNamesR = (state, action) => {
    const { profileNames } = action.payload

    if (state.profileNames.length !== profileNames.length) {
        state.profileNames = profileNames
        return state
    }
}

const updateCheckedLoginR = (state, action) => {
    const { checkedLogin } = action.payload

    if (state.checkedLogin !== checkedLogin) {
        state.checkedLogin = checkedLogin
    }

    return state
}

const updateConnectionErrorR = (state, action) => {
    const { connectionError } = action.payload

    if (state.connectionError !== connectionError) {
        state.connectionError = connectionError
        return state
    }
}

const updateCheckingLoginR = (state, action) => {
    const { checkingLogin } = action.payload

    if (state.checkLogin !== checkingLogin) {
        state.checkingLogin = checkingLogin
        return state
    }
}

const useLoginStatusReducer = () => {
    const { name, reducer, actions, caseReducers } = createSlice({
        name: 'login-status',
        initialState: initialLoginStatusState,
        reducers: {
            updateIsLoggedIn: updateIsLoggedInR,
            updateProfilesFetched: updateProfilesFetchedR,
            updateProfileNames: updateProfileNamesR,
            updateCheckedLogin: updateCheckedLoginR,
            updateConnectionError: updateConnectionErrorR,
            updateCheckingLogin: updateCheckingLoginR,
        },
    })

    const [state, dispatch] = useReducer(reducer, initialLoginStatusState)

    return {
        name,
        actions,
        reducer,
        state,
        dispatch,
        caseReducers,
    }
}

function LoginStatusProvider({ apiEndpoint, children }) {
    // const [isLoggedIn, setIsLoggedIn] = useState(false)
    // const [profileNames, setProfileNames] = useState([])
    // const [profilesFetched, setProfilesFetched] = useState(false)
    // const [checkedLogin, setCheckedLogin] = useState(false)
    // const [connectionError, setConnectionError] = useState(null)
    // const [checkingLogin, setCheckingLogin] = useState(true)
    const {
        state: {
            isLoggedIn,
            profilesFetched,
            profileNames,
            checkedLogin,
            connectionError,
            checkingLogin,
        },
        actions: {
            updateIsLoggedIn,
            updateProfilesFetched,
            updateProfileNames,
            updateCheckedLogin,
            updateConnectionError,
            updateCheckingLogin,
        },
        dispatch,
    } = useLoginStatusReducer()

    const getProfileNames = useCallback(async () => {
        try {
            const resp = await (
                await fetch(`${apiEndpoint}/api/profile-names`, {
                    method: 'GET',
                })
            ).json()
            if (!profilesFetched) {
                dispatch(updateProfilesFetched({ profilesFetched: true }))
            }

            if (profileNames.length !== (resp.profileNames || []).length) {
                dispatch(
                    updateProfileNames({
                        profileNames: resp.profileNames || [],
                    }),
                )
            }
        } catch (err) {
            console.log(err)
            dispatch(updateProfileNames({ profileNames: [] }))
        }
    }, [
        dispatch,
        updateProfileNames,
        updateProfilesFetched,
        apiEndpoint,
        profileNames.length,
        profilesFetched,
    ])

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
            dispatch(updateIsLoggedIn({ isLoggedIn: true }))
        },
        [apiEndpoint, isLoggedIn, updateIsLoggedIn, getProfileNames, dispatch],
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
        dispatch(updateProfilesFetched({ profilesFetched: false }))
        // NOTE: Need to reset all context states here
        await getProfileNames()
        dispatch(updateIsLoggedIn({ isLoggedIn: false }))
    }, [
        apiEndpoint,
        isLoggedIn,
        updateIsLoggedIn,
        getProfileNames,
        dispatch,
        updateProfilesFetched,
    ])

    const checkLogin = useCallback(async () => {
        try {
            dispatch(updateCheckingLogin({ checkingLogin: true }))
            const resp = await fetch(`${apiEndpoint}/api/check-login`, {
                method: 'POST',
            })
            const jsonResp = await resp.text()
            if (jsonResp === 'true') {
                dispatch(updateIsLoggedIn({ isLoggedIn: true }))
            }
            dispatch(updateCheckingLogin({ checkingLogin: false }))
            dispatch(updateConnectionError({ connectionError: false }))
            await getProfileNames()
            return true
        } catch (err) {
            dispatch(updateConnectionError({ connectionError: err.message }))
            dispatch(updateCheckingLogin({ checkingLogin: false }))
            return err.message
        }
    }, [
        apiEndpoint,
        getProfileNames,
        isLoggedIn,
        updateCheckingLogin,
        updateConnectionError,
        updateIsLoggedIn,
        dispatch,
    ])

    const initialCheckLogin = useCallback(async () => {
        if (!checkedLogin && !connectionError) {
            try {
                await checkLogin()
                dispatch(updateCheckedLogin({ checkedLogin: true }))
            } catch (err) {
                dispatch(updateCheckedLogin({ checkedLogin: true }))
            }
        }
    }, [
        checkedLogin,
        connectionError,
        checkLogin,
        dispatch,
        updateCheckedLogin,
    ])

    useEffect(() => {
        initialCheckLogin()
    }, [])

    return (
        <LoginStatusContext.Provider
            value={{
                signup,
                login,
                logout,
                checkLogin,
                isLoggedIn,
                profileNames,
                profilesFetched,
                connectionError,
                checkingLogin,
            }}
        >
            {children}
        </LoginStatusContext.Provider>
    )
}

export default LoginStatusProvider
