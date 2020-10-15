import React from 'react'
import ReactDOM from 'react-dom'
import './index.css'
import * as serviceWorker from './serviceWorker'
import './braidjs/braid-src'
import App from './App'
let Braid = window.Braid

export let braidClient

;(async function() {
    braidClient = Braid.createPeer({
        identity: Braid.identity.random(),
        httpHost: 'http://localhost:3001',
        onFoundPeersCallback: (peers) => {},
    })
    await braidClient.authorize()

    ReactDOM.render(
        <React.StrictMode>
            <App />
        </React.StrictMode>,
        document.getElementById('root')
    )

    // If you want your app to work offline and load faster, you can change
    // unregister() to register() below. Note this comes with some pitfalls.
    // Learn more about service workers: https://bit.ly/CRA-PWA
    serviceWorker.register()
})()
