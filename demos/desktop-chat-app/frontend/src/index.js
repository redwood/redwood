import React from 'react'
import ReactDOM from 'react-dom'
import { ThemeProvider } from 'styled-components'
import App from './App'
import reportWebVitals from './reportWebVitals'
import RedwoodProvider from './contexts/Redwood'
import ModalsProvider from './contexts/Modals'
import APIProvider from './contexts/API'

import './index.css'
import theme from './theme'

ReactDOM.render(
    <React.StrictMode>
        <ThemeProvider theme={theme}>
            <RedwoodProvider>
                <APIProvider>
                    <ModalsProvider>
                        <App />
                    </ModalsProvider>
                </APIProvider>
            </RedwoodProvider>
        </ThemeProvider>
    </React.StrictMode>,
    document.getElementById('root')
)

// If you want to start measuring performance in your app, pass a function
// to log results (for example: reportWebVitals(console.log))
// or send to an analytics endpoint. Learn more: https://bit.ly/CRA-vitals
reportWebVitals()
