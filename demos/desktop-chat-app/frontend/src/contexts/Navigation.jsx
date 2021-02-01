import React, { createContext, useCallback, useState, useEffect } from 'react'

export const Context = createContext({
    selectedStateURI: null,
    selectedServer: null,
    selectedRoom: null,
    navigate: () => {},
})

function Provider({ children }) {
    const [selectedServer, setServer] = useState(null)
    const [selectedRoom, setRoom] = useState(null)

    let selectedStateURI = (selectedServer && selectedRoom) ? `${selectedServer}/${selectedRoom}` : null

    function navigate(selectedServer, selectedRoom) {
        console.log('navigate', selectedServer, selectedRoom)
        setServer(selectedServer)
        setRoom(selectedRoom)
    }
    console.log('selectedStateURI', selectedStateURI)

    return (
      <Context.Provider value={{ selectedStateURI, selectedServer, selectedRoom, navigate }}>
          {children}
      </Context.Provider>
    )
}

export default Provider