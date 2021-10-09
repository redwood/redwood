// import RedwoodProvider from './Redwood' // RedwoodProviderWrapper
import { RedwoodProvider } from 'redwood-client-test/react'
import AddressBookProvider, { AddressBookContext } from './AddressBook'
import APIProvider, { APIContext } from './API'
import LoginStatusProvider, { LoginStatusContext } from './LoginStatus'
import ModalsProvider, { ModalsContext } from './Modals'
import NavigationProvider, { NavigationContext } from './Navigation'
import PeersProvider, { PeersContext } from './Peers'
import ServerAndRoomInfoProvider, {
    ServerAndRoomInfoContext,
} from './ServerAndRoomInfo'
import LoadingProvider, { LoadingContext } from './Loading'

import * as contextUtils from './utils'

export const { ComposeComponents, combineProviders, withContextProvider } =
contextUtils

// *GUIDELINE: Contexts that need to be imported by hooks. Avoid using Context outside of hooks.
export const Contexts = {
    AddressBookContext,
    APIContext,
    LoginStatusContext,
    ModalsContext,
    NavigationContext,
    PeersContext,
    ServerAndRoomInfoContext,
    LoadingContext,
}

/* *GUIDELINE: 
	- Anytime that you create a Provider/Context always give the functional component or const a descriptive name
		- Invalid:  function Provider(props) { ... returns <Context.Provider> }
		- Valid: function RedwoodProvider(props) { ... returns <RedwoodContext.Provider> } 
	- Add Providers below that you want to wrap <Main />
	- *NOTICE: Order matters when passing in Providers
*/
export const MainProviders = {
    RedwoodProvider,
    LoadingProvider,
    APIProvider,
    NavigationProvider,
    AddressBookProvider,
    ServerAndRoomInfoProvider,
    PeersProvider,
    ModalsProvider,
    // LoginStatusProvider, // Left out because this lives on main <App />
}

/* *GUIDELINE:
	- If you are using ComposeComponents, pass props here { [ComponentName]: { ...propsHere }, ... }
	- The key on ComposeComponents is componentProps
	- If componentProps aren't passing properly changes are that you aren't writing the correct component name
	- To see the rendered component names pass the prop `logComponentNames` to ComposeComponents
*/
export const mainProviderProps = {
    RedwoodProvider: {
        httpHost: '',
        rpcEndpoint: '',
    },
    // LoadingProvider: {
    //     isLoading: true,
    // },
    // PeersProvider: {
    //     peers: [],
    // },
    // [ComponentName]: {
    // 	...props values here
    // }
}

export default {
    ...Contexts,
    ...MainProviders,
    LoginStatusProvider,
}
