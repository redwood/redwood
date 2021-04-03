import React, { useState, useCallback, useEffect } from 'react'
import {
    BrowserRouter as Router,
    Switch,
    Route,
} from 'react-router-dom'
import { ThemeProvider } from 'styled-components'
import { RedwoodProvider } from 'redwood/dist/main/react'

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
    let [httpHost, setHttpHost] = useState()
    let [rpcEndpoint, setRpcEndpoint] = useState()
    let { isLoggedIn } = useLoginStatus()

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
                useWebsocket={true}
            >
                <APIProvider>
                    <NavigationProvider>
                        <ServerAndRoomInfoProvider>
                            <PeersProvider>
                                <ModalsProvider>
                                    <Router>
                                        <Switch>
                                            <Route exact path="/">
                                                <Main />
                                            </Route>
                                            <Route path="/signin">
                                                <SignIn />
                                            </Route>
                                            <Route path="/signup">
                                                <SignUp />
                                            </Route>
                                            <Route path="/profiles">
                                                <Profiles />
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