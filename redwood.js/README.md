

# Redwood.js

## Redwood client

WIP

## React hooks

```tsx
import React, { useRef } from 'react'
import Redwood from '@redwood.dev/client'
import { RedwoodProvider, useRedwood, useStateTree } from '@redwood.dev/client/react'

const identity = Redwood.identity.random()

function App() {
    return (
        <RedwoodProvider
            httpHost="http://localhost:8080"
            identity={identity}
        >
            <ChatRoom />
        </RedwoodProvider>
    )
}

function ChatRoom() {
    const { redwoodClient } = useRedwood()
    const chatRoom = useStateTree('chat.redwood.dev/general')
    const textInput = useRef()

    function onClickSend() {
        redwoodClient.put({
            stateURI: 'chat.redwood.dev/general'
            patches: [
                '.messages[0:0] = ' + Redwood.utils.JSON.stringify({
                    sender: identity.address,
                    text:   textInput.current.value,
                }),
            ],
        })
    }

    return (
        <div>
            {chatRoom.messages.map(msg => (
                <div>
                    <div>{msg.sender}</div>
                    <div>{msg.text}</div>
                </div>
            ))}

            <input ref={textInput} />
            <button onClick={onClickSend}>Send</button>
        </div>
    )
}
```