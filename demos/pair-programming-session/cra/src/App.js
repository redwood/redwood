import React, { useReducer, useEffect } from 'react'
import Chat from './Chat'
import Editor from './Editor'
import Video from './Video'

function App() {
    return (
        <div className="App">
            <div style={{ display: 'flex' }}>
                <div style={{ flexGrow: 1, flexShrink: 1 }}>
                    <Editor />
                </div>
                <div style={{ width: '18vw' }}>
                    <Chat />
                </div>
            </div>
            <div style={{ display: 'flex' }}>
                <div style={{ width: '100%', height: '20vh' }}>
                    <Video />
                </div>
            </div>
        </div>
    )
}

export default App
