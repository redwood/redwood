import React, { createContext, useState, useDebugValue } from 'react'

export const Context = createContext({
    selectedStateURI: null,
    selectedServer: null,
    selectedRoom: null,
    navigate: () => {},
})

function Provider({ children }) {
    const [selectedServer, setServer] = useState(null)
    const [selectedRoom, setRoom] = useState(null)

    const selectedStateURI =
        selectedServer && selectedRoom
            ? `${selectedServer}/${selectedRoom}`
            : null

    useDebugValue({ selectedServer, selectedRoom, selectedStateURI })

    const navigate = (server, room) => {
        setServer(server)
        setRoom(room)
    }

    return (
        <Context.Provider
            value={{
                selectedStateURI,
                selectedServer,
                selectedRoom,
                navigate,
            }}
        >
            {children}
        </Context.Provider>
    )
}

export default Provider
