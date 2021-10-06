import { useState, useEffect } from 'react'
import { RedwoodProvider as RedwoodJSLibProvider } from '../components/redwood.js/dist/main/react'
// import { LoginStatusContext } from './LoginStatus'

// *GUIDELINE: Any time you need to wrap a 3rd party Context Provider follow this approach
// RedwoodJSLibProvider httpHost and rpcEndpoint props are now dynamically changed whenever
// isLoggedIn is changed to true or false
function RedwoodProvider({ children }) {
    // const [httpHost, setHttpHost] = useState()
    // const [rpcEndpoint, setRpcEndpoint] = useState()

    // Set httpHost and rpcEndpoint anytime user is logged in
    // NOTE: On logout ensure that all contexts are reset (perhaps should happen in logout function)
    // useEffect(() => {
    //     if (isLoggedIn) {
    //         setHttpHost('http://localhost:8080')
    //         setRpcEndpoint('http://localhost:8081')
    //     } else {
    //         setHttpHost()
    //         setRpcEndpoint()
    //     }
    // }, [isLoggedIn, setHttpHost, setRpcEndpoint])

    return <RedwoodJSLibProvider>{children}</RedwoodJSLibProvider>
}

export default RedwoodProvider
