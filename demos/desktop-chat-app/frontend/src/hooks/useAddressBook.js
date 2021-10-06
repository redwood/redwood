import { useMemo, useContext } from 'react'
import { AddressBookContext } from '../contexts/AddressBook'

function useAddressBook() {
    const addressBook = useContext(AddressBookContext)
    return useMemo(() => addressBook, [addressBook])
}

export default useAddressBook
