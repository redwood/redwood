import { createContext, useMemo } from 'react'
import useStateTree from '../hooks/useStateTree'

export const AddressBookContext = createContext({})

function AddressBookProvider({ children }) {
    const addressBook = useStateTree('chat.local/address-book')

    const safeAddressBook = useMemo(
        () => (addressBook && addressBook.value ? addressBook.value : {}),
        [addressBook],
    )

    return (
        <AddressBookContext.Provider value={safeAddressBook}>
            {children}
        </AddressBookContext.Provider>
    )
}

export default AddressBookProvider
