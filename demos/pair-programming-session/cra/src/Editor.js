import React, { useState, useEffect, useRef } from 'react'
import { braidClient } from './App'
import { keyboardCharMap, keyboardNameMap } from './keyboard-map'

let Braid = window.Braid

function Editor(props) {
    let [editorText, setEditorText] = useState('')
    let [mostRecentStartEnd, setMostRecentStartEnd] = useState([0, 0])
    let editorRef = useRef(null)

    // Set editor text once on first render
    useEffect(() => setEditorText(props.state.text.value), [])

    // Set editor text when the text prop changes
    useEffect(() => {
        setEditorText(props.state.text.value)
        if (!props.tx) {
            return
        } else if (!editorRef.current) {
            return
        }

        let n = 0
        for (let patch of props.tx.patches) {
            let parsed = Braid.sync9.parse_change(patch)
            if (parsed.range && parsed.range[0] <= editorRef.current.selectionStart) {
                n += parsed.val.length - (parsed.range[1] - parsed.range[0])
            }
        }
        let selectionStart = mostRecentStartEnd[0] + n
        let selectionEnd   = mostRecentStartEnd[1] + n
        setMostRecentStartEnd([selectionStart, selectionEnd])
    }, [props.state.text.value])

    useEffect(() => {
        editorRef.current.setSelectionRange(...mostRecentStartEnd)
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
        let tx = {
            id: window.Braid.utils.randomID(),
            parents: props.leaves,
            stateURI: 'p2pair.local/editor',
            patches: [
                '.text.value[' + selectionStart + ':' + selectionEnd + '] = ' + JSON.stringify(char),
            ],
        }
        braidClient.put(tx)

        let editorText = evt.target.value.substring(0, evt.target.selectionStart) + char + evt.target.value.substring(evt.target.selectionEnd)
        setEditorText(editorText)
        setMostRecentStartEnd([selectionStart + 1, selectionStart + 1])
    }

    function onChange(evt) {}

    function onSelect(evt) {
        setMostRecentStartEnd([evt.target.selectionStart, evt.target.selectionEnd])
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

