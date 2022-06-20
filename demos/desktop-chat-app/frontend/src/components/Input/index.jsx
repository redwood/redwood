import React, { useState, useCallback } from 'react'
import styled from 'styled-components'

const SInput = styled.input`
    width: calc(100% - 24px);
    border: none;
    font-size: 16px;
    border-radius: 12px;
    background-color: ${props => props.theme.color.grey[100]};
    color: ${props => props.theme.color.white};
    padding: 6px 12px;
    &:focus {
        outline: none;
    }
`

function Input(props, ref) {
    const { onEnter, onKeyDown } = props

    const onKeyDownInner = useCallback(event => {
        if (event.key === 'Enter' && !!onEnter) {
            onEnter(event)
        }
        if (onKeyDown) {
            onKeyDown(event)
        }

    }, [onKeyDown, onEnter])

    return (
        <SInput {...props} ref={ref} onKeyDown={onKeyDownInner} />
    )
}

const SInputWrapper = styled.div`
    display: flex;
    flex-direction: column;
    width: ${props => props.width ? props.width : '100%'};
`

const SInputLabel = styled.label`
    font-size: 10px;
    margin-bottom: 6px;
    color: rgba(255, 255, 255, .8);
`

export function InputLabel(props) {
    return (
        <SInputWrapper width={props.width} className={props.className}>
            <SInputLabel>{props.label}</SInputLabel>
            {props.children}
        </SInputWrapper>
    )
}

export default React.forwardRef(Input)
