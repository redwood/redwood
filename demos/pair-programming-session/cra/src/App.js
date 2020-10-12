import React, { useReducer, useEffect } from 'react'
import logo from './logo.svg'
import './braidjs/braid-src'
import Chat from './Chat'
import Editor from './Editor'
console.log('BRAID', window.Braid)
let Braid = window.Braid

export let braidClient

function App() {

    const reducer = (state, action) => {
        switch (action.type) {
        case 'NEW_STATE_CHAT':
            console.log('payload ~>', action.payload)
            return { ...state, chat: action.payload }
        case 'NEW_STATE_EDITOR':
            return { ...state, editor: action.payload }
        default:
            throw new Error('Unexpected action')
        }
    }

    const [state, dispatch] = useReducer(reducer, {
        chat: { init: true },
        editor: { init: true },
    })

    useEffect(() => {
        braidClient = Braid.createPeer({
            identity: Braid.identity.random(),
            httpHost: 'http://localhost:3001',
            onFoundPeersCallback: (peers) => {},
        })
        braidClient.authorize().then(() => {
            braidClient.subscribeStates('p2pair.local/chat', '/', (err, newState) => {
                console.log('chat ~>', err, newState)
                if (err) {
                    console.error(err)
                    return
                }
                dispatch({ type: 'NEW_STATE_CHAT', payload: newState })
            })

            braidClient.subscribeStates('p2pair.local/editor', '/', (err, newState) => {
                console.log('editor ~>', err, newState)
                if (err) {
                    console.error(err)
                    return
                }
                dispatch({ type: 'NEW_STATE_EDITOR', payload: newState })
            })
        })
    }, [])

    return (
        <div className="App">
            <Chat {...state.chat} />
            <Editor {...state.editor} />
        </div>
    )
}

export default App
