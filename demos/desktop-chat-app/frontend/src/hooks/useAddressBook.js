import { useRef } from 'react'
import { useStateTree } from '../components/redwood.js/dist/main/react'

function useAddressBook() {
    const defaultValue = useRef({})
    const addressBook = useStateTree('chat.local/address-book')
    if (addressBook && addressBook.value) {
        return addressBook.value
    }
    return defaultValue.current
}

export default useAddressBook
