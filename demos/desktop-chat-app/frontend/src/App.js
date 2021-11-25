import React, { useState, useEffect, useRef } from 'react'
import { BrowserRouter as Router, Switch, Route } from 'react-router-dom'
import { ThemeProvider } from 'styled-components'
import { RedwoodProvider } from './components/redwood.js/dist/main/react'

import ModalsProvider from './contexts/Modals'
import APIProvider from './contexts/API'
import NavigationProvider from './contexts/Navigation'
import PeersProvider from './contexts/Peers'
import ServerAndRoomInfoProvider from './contexts/ServerAndRoomInfo'
import useLoginStatus from './hooks/useLoginStatus'
import theme from './theme'

import Main from './Main'
import SignIn from './components/Account/SignIn'
import SignUp from './components/Account/SignUp'
import Profiles from './components/Account/Profiles'

function App() {
    const [httpHost, setHttpHost] = useState()
    const [rpcEndpoint, setRpcEndpoint] = useState()
    const { isLoggedIn, profileNames } = useLoginStatus()
    const renderCountRef = useRef(1)

    useEffect(() => {
        if (isLoggedIn) {
            setHttpHost('http://localhost:8080')
            setRpcEndpoint('http://localhost:8081')
        } else {
            setHttpHost()
            setRpcEndpoint()
        }
    }, [isLoggedIn, setHttpHost, setRpcEndpoint])

    return (
        <ThemeProvider theme={theme}>
            <RedwoodProvider
                httpHost={httpHost}
                rpcEndpoint={rpcEndpoint}
                useWebsocket
            >
                <APIProvider>
                    <NavigationProvider>
                        <ServerAndRoomInfoProvider>
                            <PeersProvider>
                                <ModalsProvider>
                                    <Router>
                                        <Switch>
                                            <Route path="/signin">
                                                <SignIn
                                                    profileNames={profileNames}
                                                />
                                            </Route>
                                            <Route path="/signup">
                                                <SignUp
                                                    profileNames={profileNames}
                                                />
                                            </Route>
                                            <Route path="/profiles">
                                                <Profiles />
                                            </Route>
                                            <Route>
                                                <Main
                                                    profileNames={profileNames}
                                                    renderCountRef={
                                                        renderCountRef
                                                    }
                                                />
                                            </Route>
                                        </Switch>
                                    </Router>
                                </ModalsProvider>
                            </PeersProvider>
                        </ServerAndRoomInfoProvider>
                    </NavigationProvider>
                </APIProvider>
            </RedwoodProvider>
        </ThemeProvider>
    )
}

export default App
