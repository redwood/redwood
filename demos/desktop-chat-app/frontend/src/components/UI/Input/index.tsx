import { CSSProperties, useMemo } from 'react'
import styled, { css } from 'styled-components'

const SInputWrapper = styled.div`
    color: white;
`

const SInput = styled.input`
    color: red;
`

interface InputProps {
    onChange: () => void
    value: string
    style?: CSSProperties
    placeholder: string
}

function Input({
    placeholder = '',
    style = {},
    value,
    onChange,
}: InputProps): JSX.Element {
    return (
        <SInputWrapper>
            <SInput
                placeholder={placeholder}
                style={style}
                value={value}
                onChange={onChange}
            />
        </SInputWrapper>
    )
}

export default Input
