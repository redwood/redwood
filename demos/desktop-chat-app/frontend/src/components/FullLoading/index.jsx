import { Redirect } from 'react-router-dom'

function FullLoading({ isLoading, isLoggedIn }) {
    if (!isLoading) {
        if (!isLoggedIn) {
            return <Redirect exact to="/profiles" />
        }
        return <Redirect exact to="/" />
    }

    return <div>Loading</div>
}

export default FullLoading
