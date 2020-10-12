import React, { useState, useEffect } from 'react'
import { braidClient } from './App'
import { keyboardCharMap, keyboardNameMap } from './keyboard-map'

let Braid = window.Braid

function Editor(props) {
    let [editorText, setEditorText] = useState('')

    console.log('props', props)

    useEffect(() => {
        console.log('callback ~>', ((props.state || {}).text || {}).value || '')
        setEditorText(((props.state || {}).text || {}).value || '')
    }, [])

    useEffect(() => {
        console.log('callback ~>', ((props.state || {}).text || {}).value || '')
        setEditorText(((props.state || {}).text || {}).value || '')
    }, [((props.state || {}).text || {}).value || ''])

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
        // let char = evt.nativeEvent.data
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
    }

    function onChange(evt) {
        // let editorText = evt.target.value.substring(0, evt.target.selectionStart) + char + evt.target.value.substring(evt.target.selectionEnd)
        setEditorText(evt.target.value)
    }

    return (
        <textarea onKeyDown={onKeyDown} onChange={onChange} value={editorText} />
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

