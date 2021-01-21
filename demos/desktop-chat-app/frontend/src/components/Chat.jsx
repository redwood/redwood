import React, { useState } from 'react'
import styled from 'styled-components'
import useBraid from '../hooks/useBraid'
import rpcFetch from '../utils/rpcFetch'
import Sidebar from './Sidebar'
import * as api from '../api'

function Chat({ stateURI, className }) {
    const { appState, nodeAddress, registry } = useBraid()
    const [messageText, setMessageText] = useState('')

    function onChangeMessageText(e) {
        setMessageText(e.target.value)
    }

    async function onClickSend() {
        await api.sendMessage(stateURI, nodeAddress.Address, appState, messageText)
    }

    console.log(stateURI)

    if (!stateURI) {
        return <div className={className}></div>
    }

    let messages = (appState[stateURI] || {}).messages || []

    return (
        <div className={className}>
            <div>
                {messages.map(msg => (
                    <div>
                        <div>{msg.sender}:</div>
                        <div>{msg.text}</div>
                    </div>
                ))}
            </div>

            <input onChange={onChangeMessageText} value={messageText} />
            <button onClick={onClickSend}>Send</button>
        </div>
    )
}

export default Chat
