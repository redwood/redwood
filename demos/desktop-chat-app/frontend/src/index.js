import React from 'react'
import ReactDOM from 'react-dom'
import { ThemeProvider } from 'styled-components'
import App from './App'
import reportWebVitals from './reportWebVitals'
import { RedwoodProvider } from 'redwood/dist/main/react'
import ModalsProvider from './contexts/Modals'
import APIProvider from './contexts/API'
import NavigationProvider from './contexts/Navigation'

import './index.css'
import theme from './theme'

ReactDOM.render(
        <ThemeProvider theme={theme}>
            <RedwoodProvider
                httpHost="http://localhost:8080"
                rpcEndpoint="http://localhost:8081"
                useWebsocket={true}
            >
                <APIProvider>
                    <NavigationProvider>
                        <ModalsProvider>
                            <App />
                        </ModalsProvider>
                    </NavigationProvider>
                </APIProvider>
            </RedwoodProvider>
        </ThemeProvider>,
    document.getElementById('root')
)

// If you want to start measuring performance in your app, pass a function
// to log results (for example: reportWebVitals(console.log))
// or send to an analytics endpoint. Learn more: https://bit.ly/CRA-vitals
reportWebVitals()
