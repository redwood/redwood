import React, { useState } from 'react'
import { braidClient } from './App'

function Chat(props) {
    let [messageText, setMessageText] = useState('')

    function onClickSend() {
        const tx = {
            id: window.Braid.utils.randomID(),
            parents: props.leaves,
            stateURI: 'p2pair.local/chat',
            patches: [
                '.messages.value[' + (props.state.messages.value.length || 0) + ':' + (props.state.messages.value.length || 0) + '] = ' + window.Braid.utils.JSON.stringify([{
                    // attachment: attachment,
                    sender: braidClient.identity.address.toLowerCase(),
                    text: messageText,
                }]),
            ],
        }
        braidClient.put(tx)
    }

    const messages = (((props.state || {}).messages || {}).value || [])

    let json = JSON.stringify(props.state, null, 4)
    return (
        <div>
            <div>
                <div>
                    {messages.map(msg => (
                        <div>
                            <div style={{fontWeight: 'bold'}}>{msg.sender}:</div>
                            <div>{msg.text}</div>
                        </div>
                    ))}
                </div>
                <input type="text" onChange={(evt) => setMessageText(evt.target.value)} value={messageText} />
                <button onClick={onClickSend}>Send</button>
            </div>
        </div>
    )
}

export default Chat
