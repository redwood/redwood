import { createContext, useState, useDebugValue, useMemo } from 'react'

export const NavigationContext = createContext({
    selectedStateURI: null,
    selectedServer: null,
    selectedRoom: null,
    navigate: () => {},
})

function NavigationProvider({ children }) {
    const [selectedServer, setServer] = useState(null)
    const [selectedRoom, setRoom] = useState(null)

    const selectedStateURI = useMemo(
        () =>
            selectedServer && selectedRoom
                ? `${selectedServer}/${selectedRoom}`
                : null,
        [selectedRoom, selectedServer],
    )

    useDebugValue({ selectedServer, selectedRoom, selectedStateURI })

    const navigate = (server, room) => {
        setServer(server)
        setRoom(room)
    }

    return (
        <NavigationContext.Provider
            value={{
                selectedStateURI,
                selectedServer,
                selectedRoom,
                navigate,
            }}
        >
            {children}
        </NavigationContext.Provider>
    )
}

export default NavigationProvider
