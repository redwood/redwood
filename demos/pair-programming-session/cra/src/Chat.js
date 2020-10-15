import React, { useState, useEffect } from 'react'
import { makeStyles } from '@material-ui/core/styles'
import { Card, TextField, Button } from '@material-ui/core'
import { braidClient } from './index'
let Braid = window.Braid

function Chat() {
    let [messageText, setMessageText] = useState('')
    let [state, setState] = useState({ tree: { messages: { value: [] } } })

    let { leaves, tree } = state
    let { messages: { value: messages } } = tree
    let classes = useStyles()

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
        <section className={classes.container}>
            <div id="container-chat-messages">
                {messages.map(msg => (
                    <div className={classes.messageCard}>
                        <div style={{ fontWeight: 'bold', textOverflow: 'ellipsis', overflow: 'hidden' }}>
                            {msg.sender}:
                        </div>
                        <div>{msg.text}</div>
                    </div>
                ))}
            </div>
            <div className={classes.spacer} />
            <div>
                <TextField onChange={(evt) => setMessageText(evt.target.value)} value={messageText} />
                <Button variant="contained" color="primary" onClick={onClickSend}>Send</Button>
            </div>
        </section>
    )
}

var useStyles = makeStyles({
    container: {
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
    },
    messageCard: {
        marginBottom: 16,
        padding: 16,
    },
    spacer: {
        flexGrow: 1,
    },
})

export default Chat
