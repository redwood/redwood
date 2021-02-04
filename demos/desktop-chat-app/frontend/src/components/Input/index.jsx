import React, { useState } from 'react'
import styled from 'styled-components'

const SInput = styled.input`
    width: 100%;
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

const SInputWrapper = styled.div`
    display: flex;
    flex-direction: column;
    width: ${props => props.width ? props.width : '300px'};
    input {
        width: calc(100% - 24px);
    }
`

const SInputLabel = styled.label`
    font-size: 10px;
    margin-bottom: 6px;
`

function Input(props) {
    if (props.label) {
        return (
            <SInputWrapper width={props.width}>
                <SInputLabel>{props.label}</SInputLabel>
                <SInput {...props} />     
            </SInputWrapper>
        )
    }

    return <SInput {...props} />
}

export default Input
