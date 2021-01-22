import React, { useState } from 'react'
import styled from 'styled-components'

const SInput = styled.input`
    width: 100%;
    border-top: none;
    border-left: none;
    border-right: none;
    font-size: 16px;
`

function Input(props) {
    return <SInput {...props} />
}

export default Input
