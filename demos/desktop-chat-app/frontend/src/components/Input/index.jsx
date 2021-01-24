import React, { useState } from 'react'
import styled from 'styled-components'

const SInput = styled.input`
    width: 100%;
    border: none;
    font-size: 16px;
    border-radius: 12px;
    background-color: ${props => props.theme.color.grey[300]};
    color: ${props => props.theme.color.white};
    padding: 6px 12px;

    &:focus {
        outline: none;
    }
`

function Input(props) {
    return <SInput {...props} />
}

export default Input
