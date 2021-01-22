import React, { useState } from 'react'
import styled from 'styled-components'

const SButton = styled.div`
    background: ${props => props.theme.color.kindOfBlue};
    color: white;
    text-align: center;
    padding: 10px;
    margin: 4px;
    font-weight: 500;
    font-size: 16px;
    border-radius: 5px;
    width: ${props => props.width || '100px'};
    box-shadow: ${props => props.mouseDown ? 'none' : '-4px 5px 7px -4px rgba(0,0,0,0.49)'};
    cursor: pointer;
    user-select: none;
    transition: background 120ms linear,
                            background-color 65ms linear,
                            box-shadow 35ms linear;

    &:hover {
        background: ${props => props.mouseDown ? '#494975' : '#6868a0'};
    }
`

function Button(props) {
    let [mouseDown, setMouseDown] = useState(false)

    function onMouseDown() {
        setMouseDown(true)
    }
    function onMouseUp() {
        setMouseDown(false)
    }

    if (props.href) {
        return (
            <a href={props.href} target="_blank">
                <SButton {...props} mouseDown={mouseDown} onMouseDown={onMouseDown} onMouseUp={onMouseUp} />
            </a>
        )
    }
    return (
        <SButton {...props} mouseDown={mouseDown} onMouseDown={onMouseDown} onMouseUp={onMouseUp} />
    )
}

export default Button