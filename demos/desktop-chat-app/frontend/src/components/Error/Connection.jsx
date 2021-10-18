import { Redirect } from 'react-router-dom'
import styled from 'styled-components'

import Card from '../UI/Card'

const SConnection = styled.div`
    background: ${({ theme }) => theme.color.textDark};
    width: 100vw;
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
`

function Connection({
    checkLogin,
    isLoggedIn,
    connectionError,
    checkingLogin,
}) {
    if (checkingLogin) {
        return <div>Checking login...</div>
    }
    if (!connectionError && isLoggedIn) {
        return <Redirect to="/" />
    }
    if (!connectionError && !isLoggedIn) {
        return <Redirect to="/profiles" />
    }

    return (
        <SConnection>
            <Card>
                <h2>Error with your local node.</h2>
                <h4>
                    There was problem connecting to your local node. Please
                    restart the application and try again. If this problem
                    continues please click here to file a bug report.
                </h4>
                <p>Error: {connectionError}</p>
                <button type="button" onClick={checkLogin}>
                    Click here to retry
                </button>
            </Card>
        </SConnection>
    )
}

export default Connection

// import { Redirect } from 'react-router-dom'

// function Connection({
//     checkLogin,
//     isLoggedIn,
//     connectionError,
//     checkingLogin,
// }) {
//     if (checkingLogin) {
//         return <div>Checking login...</div>
//     }
//     if (!connectionError && isLoggedIn) {
//         return <Redirect to="/" />
//     }
//     if (!connectionError && !isLoggedIn) {
//         return <Redirect to="/profiles" />
//     }

//     return (
//         <div>
//             <h2>Error with your local node.</h2>
//             <h4>
//                 There was problem connecting to your local node. Please restart
//                 the application and try again. If this problem continues please
//                 click here to file a bug report.
//             </h4>
//             <p>Error: {connectionError}</p>
//             <button type="button" onClick={checkLogin}>
//                 Click here to retry
//             </button>
//         </div>
//     )
// }

// export default Connection
