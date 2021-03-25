import React, { createContext, useCallback, useState, useEffect, useDebugValue } from 'react'

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

    useDebugValue({ selectedServer, selectedRoom, selectedStateURI })

    function navigate(selectedServer, selectedRoom) {
        setServer(selectedServer)
        setRoom(selectedRoom)
    }

    return (
      <Context.Provider value={{
          selectedStateURI,
          selectedServer,
          selectedRoom,
          navigate,
      }}>
          {children}
      </Context.Provider>
    )
}

export default Provider