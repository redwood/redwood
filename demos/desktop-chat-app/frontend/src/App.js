import { BrowserRouter as Router, Route, Switch } from 'react-router-dom'
import { ThemeProvider } from 'styled-components'

import { ComposeComponents, MainProviders, mainProviderProps } from './contexts'
import LoginStatusProvider, { LoginStatusContext } from './contexts/LoginStatus'

import SignIn from './components/Account/SignIn'
import SignUp from './components/Account/SignUp'
import Profiles from './components/Account/Profiles'
import Main from './Main'
import theme from './theme'

// @NOTE: Need to add this ConnectionError component somewhere universal
import FullLoading from './components/FullLoading'
import ConnectionError from './components/Error/Connection'

function Routes({
    isLoggedIn,
    signup,
    profileNames = [],
    login,
    profilesFetched,
    connectionError,
    checkLogin,
    checkingLogin,
}) {
    return (
        <>
            <Route path="/connection-error">
                <ConnectionError
                    isLoggedIn={isLoggedIn}
                    connectionError={connectionError}
                    checkLogin={checkLogin}
                    checkingLogin={checkingLogin}
                />
            </Route>
            <Route path="/loading">
                <FullLoading
                    isLoading={checkingLogin}
                    isLoggedIn={isLoggedIn}
                />
            </Route>
            <Route path="/signin">
                <SignIn
                    profileNames={profileNames}
                    login={login}
                    connectionError={connectionError}
                    isLoggedIn={isLoggedIn}
                    checkingLogin={checkingLogin}
                />
            </Route>
            <Route path="/signup">
                <SignUp
                    isLoggedIn={isLoggedIn}
                    signup={signup}
                    login={login}
                    connectionError={connectionError}
                    profileNames={profileNames}
                    checkingLogin={checkingLogin}
                />
            </Route>
            <Route path="/profiles">
                <Profiles
                    isLoggedIn={isLoggedIn}
                    signup={signup}
                    login={login}
                    connectionError={connectionError}
                    profileNames={profileNames}
                    checkingLogin={checkingLogin}
                />
            </Route>
            <Route exact path="/">
                <ComposeComponents
                    components={Object.values(MainProviders)}
                    componentProps={mainProviderProps}
                >
                    <Main
                        isLoggedIn={isLoggedIn}
                        profilesFetched={profilesFetched}
                        connectionError={connectionError}
                        profileNames={profileNames}
                    />
                </ComposeComponents>
            </Route>
        </>
    )
}

function App() {
    return (
        <Router>
            <LoginStatusProvider apiEndpoint="http://localhost:54231">
                <ThemeProvider theme={theme}>
                    <LoginStatusContext.Consumer>
                        {(loginStatus) => (
                            <Switch>
                                <Routes {...loginStatus} />
                            </Switch>
                        )}
                    </LoginStatusContext.Consumer>
                </ThemeProvider>
            </LoginStatusProvider>
        </Router>
    )
}

export default App
