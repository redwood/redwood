import React, { useState, useEffect, useRef } from 'react'
import { braidClient } from './index'
import { keyboardCharMap, keyboardNameMap } from './keyboard-map'
let Braid = window.Braid

function Editor() {
    let [state, setState] = useState({ tree: { text: { value: '' } } })
    let [editorText, setEditorText] = useState('')
    let [mostRecentSelection, setMostRecentSelection] = useState([0, 0])
    let editorRef = useRef(null)
    console.log('state', state)

    let { tx, leaves, tree } = state
    let { text: { value: text } } = tree

    useEffect(() => {
        braidClient.subscribe({
            stateURI: 'p2pair.local/editor',
            keypath:  '/',
            txs:      true,
            states:   true,
            fromTxID: Braid.utils.genesisTxID,
            callback: (err, { tx: newTx, state: newTree, leaves: newLeaves } = {}) => {
                console.log('editor ~>', err, {newTx, newTree, newLeaves})
                if (err) return console.error(err)
                setState({
                    tx:     newTx,
                    tree:   { ...tree, ...newTree },
                    leaves: newLeaves || [],
                })
            },
        })
    }, [])


    // Set editor text once on first render
    useEffect(() => setEditorText(text), [])

    // Set editor text when the text prop changes
    useEffect(() => {
        setEditorText(text)
        if (!tx) {
            return
        } else if (!editorRef.current) {
            return
        }

        let n = 0
        for (let patch of tx.patches) {
            let parsed = Braid.sync9.parse_change(patch)
            if (parsed.range && parsed.range[0] <= editorRef.current.selectionStart) {
                n += parsed.val.length - (parsed.range[1] - parsed.range[0])
            }
        }
        let selectionStart = mostRecentSelection[0] + n
        let selectionEnd   = mostRecentSelection[1] + n
        setMostRecentSelection([selectionStart, selectionEnd])
    }, [text])

    useEffect(() => {
        editorRef.current.setSelectionRange(...mostRecentSelection)
    })

    function onKeyDown(evt) {
        if (['UP', 'DOWN', 'LEFT', 'RIGHT'].indexOf(keyboardNameMap[evt.keyCode]) > -1) {
            return
        }

        evt.stopPropagation()
        evt.preventDefault()
        let char = getPressedChar(evt)
        if (char === '') {
            return
        }
        let selectionStart = (evt.target.selectionStart || 0)
        let selectionEnd   = (evt.target.selectionEnd   || 0)
        braidClient.put({
            id: window.Braid.utils.randomID(),
            parents: leaves,
            stateURI: 'p2pair.local/editor',
            patches: [
                '.text.value[' + selectionStart + ':' + selectionEnd + '] = ' + JSON.stringify(char),
            ],
        })

        let editorText = evt.target.value.substring(0, evt.target.selectionStart) + char + evt.target.value.substring(evt.target.selectionEnd)
        setEditorText(editorText)
        setMostRecentSelection([selectionStart + 1, selectionStart + 1])
    }

    function onChange(evt) {}

    function onSelect(evt) {
        setMostRecentSelection([evt.target.selectionStart, evt.target.selectionEnd])
    }

    return (
        <section id="section-editor">
            <h2>Code editor</h2>
            <textarea
                ref={editorRef}
                onKeyDown={onKeyDown}
                onSelect={onSelect}
                onChange={onChange}
                value={editorText}
                style={{ width: '80vw', height: '90vh' }}
            />
        </section>
    )
}

export default Editor

function getPressedChar(event) {
    var iCol = 0
    if (event.shiftKey) {
        iCol = 1
    }
    return keyboardCharMap[event.keyCode][iCol]
}

