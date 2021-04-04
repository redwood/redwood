import React from 'react'
import ReactDOM from 'react-dom'
import App from './App'
import reportWebVitals from './reportWebVitals'
import LoginStatusProvider from './contexts/LoginStatus'

import './index.css'

ReactDOM.render(
    <LoginStatusProvider apiEndpoint="http://localhost:54231">
        <App />
    </LoginStatusProvider>
, document.getElementById('root'))

// If you want to start measuring performance in your app, pass a function
// to log results (for example: reportWebVitals(console.log))
// or send to an analytics endpoint. Learn more: https://bit.ly/CRA-vitals
reportWebVitals()
