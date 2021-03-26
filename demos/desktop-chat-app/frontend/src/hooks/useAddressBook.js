import { useRef } from 'react'
import { useStateTree } from 'redwood/dist/main/react'

function useAddressBook() {
    let defaultValue = useRef({})
    let addressBook = useStateTree('chat.local/address-book')
    if (addressBook && addressBook.value) {
        return addressBook.value
    }
    return defaultValue.current
}

export default useAddressBook
