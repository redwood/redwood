import React, { useReducer, useEffect } from 'react'
import logo from './logo.svg'
import './braidjs/braid-src'
import Chat from './Chat'
import Editor from './Editor'
let Braid = window.Braid

export let braidClient = Braid.createPeer({
    identity: Braid.identity.random(),
    httpHost: 'http://localhost:3001',
    onFoundPeersCallback: (peers) => {},
})

function App() {

    const reducer = (state, action) => {
        switch (action.type) {
        case 'NEW_STATE_CHAT':
            return {
                ...state,
                chat: action.payload,
            }

        case 'NEW_STATE_EDITOR':
            return {
                ...state,
                editor: {
                    ...state.editor,
                    ...action.payload,
                    state: {
                        ...state.editor.state,
                        ...action.payload.state,
                    },
                }
            }
        default:
            throw new Error('Unexpected action')
        }
    }

    const [state, dispatch] = useReducer(reducer, {
        chat: {},
        editor: {
            state: {
                text: {
                    value: '',
                },
            },
        },
    })

    useEffect(() => {

        braidClient.authorize().then(() => {
            // Subscribe to the chat state tree
            braidClient.subscribe({
                stateURI: 'p2pair.local/chat',
                keypath:  '/',
                txs:      true,
                states:   true,
                fromTxID: Braid.utils.genesisTxID,
                callback: (err, { tx, state, leaves } = {}) => {
                    console.log('chat ~>', err, {tx, state, leaves})
                    if (err) {
                        console.error(err)
                        return
                    }
                    dispatch({ type: 'NEW_STATE_CHAT', payload: { tx, state, leaves } })
                },
            })

            // Subscribe to the editor state tree
            braidClient.subscribe({
                stateURI: 'p2pair.local/editor',
                keypath:  '/',
                txs:      true,
                states:   true,
                fromTxID: Braid.utils.genesisTxID,
                callback: (err, { tx, state, leaves } = {}) => {
                    console.log('editor ~>', err, {tx, state, leaves})
                    if (err) {
                        console.error(err)
                        return
                    }
                    dispatch({ type: 'NEW_STATE_EDITOR', payload: { tx, state, leaves } })
                },
            })
        })
    }, [])

    return (
        <div className="App">
            <div style={{ display: 'flex' }}>
                <div style={{ flexGrow: 1, flexShrink: 1 }}>
                    <Editor {...state.editor} />
                </div>
                <div style={{ width: '18vw' }}>
                    <Chat {...state.chat} />
                </div>
            </div>
        </div>
    )
}

export default App
