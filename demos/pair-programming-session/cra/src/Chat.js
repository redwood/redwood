import React, { useState, useEffect } from 'react'
import { braidClient } from './index'
let Braid = window.Braid

function Chat(props) {
    let [messageText, setMessageText] = useState('')
    let [state, setState] = useState({ tree: { messages: { value: [] } } })

    let { leaves, tree } = state
    let { messages: { value: messages } } = tree

    useEffect(() => {
        braidClient.subscribe({
            stateURI: 'p2pair.local/chat',
            keypath:  '/',
            txs:      true,
            states:   true,
            fromTxID: Braid.utils.genesisTxID,
            callback: (err, { tx: newTx, state: newTree, leaves: newLeaves } = {}) => {
                console.log('chat ~>', err, {newTx, newTree, newLeaves})
                if (err) return console.error(err)
                setState({
                    tree:   { ...tree, ...newTree },
                    leaves: newLeaves || [],
                })
            },
        })
    }, [])


    function onClickSend() {
        braidClient.put({
            id: Braid.utils.randomID(),
            parents: leaves,
            stateURI: 'p2pair.local/chat',
            patches: [
                '.messages.value[' + (messages.length || 0) + ':' + (messages.length || 0) + '] = ' + Braid.utils.JSON.stringify([{
                    // attachment: attachment,
                    sender: braidClient.identity.address.toLowerCase(),
                    text: messageText,
                }]),
            ],
        })
    }

    return (
        <section id="section-chat">
            <h2>Chat</h2>
            <div>
                <div id="container-chat-messages">
                    {messages.map(msg => (
                        <div className="chat-message" style={{ marginBottom: 16 }}>
                            <div style={{ fontWeight: 'bold', textOverflow: 'ellipsis', overflow: 'hidden' }}>
                                {msg.sender}:
                            </div>
                            <div>{msg.text}</div>
                        </div>
                    ))}
                </div>
                <input type="text" onChange={(evt) => setMessageText(evt.target.value)} value={messageText} />
                <button onClick={onClickSend}>Send</button>
            </div>
        </section>
    )
}

export default Chat
