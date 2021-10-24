import { CSSProperties, useMemo, InputHTMLAttributes } from 'react'
import styled from 'styled-components'

const SInputLabel = styled.label`
    font-size: ${({ theme }) => `${theme.font.size.s1}px`};
    margin-bottom: 4px;
`

const SInput = styled.input<{ hasError: boolean }>`
    font-family: ${({ theme }) => theme.font.type.primary};
    color: ${({ theme }) => theme.color.text};
    background: ${({ theme }) => theme.color.accent1};
    border: 1px solid ${({ theme }) => theme.color.accent1};
    border-radius: 2px;
    font-size: ${({ theme }) => `${theme.font.size.s2}px`};
    padding: 8px 12px;
    width: 100%;
    box-sizing: border-box;
    &::placeholder {
        color: ${({ theme }) => theme.color.accent2};
    }
    border-color: ${({ hasError, theme }) =>
        hasError ? theme.color.secondary : theme.color.accent1};
    &:focus {
        outline: none;
        border-color: ${({ theme, hasError }) =>
            hasError ? theme.color.secondary : theme.color.text};
    }
    &:disabled {
        cursor: not-allowed;
        pointer-events: auto;
        background: transparent;
        &::placeholder {
            color: ${({ theme }) => theme.color.accent1};
        }
    }
`

const SInputWrapper = styled.div<{ wrapperWidth: string }>`
    color: white;
    display: flex;
    flex-direction: column;
    width: ${({ wrapperWidth }) => `${wrapperWidth}`};
`

const ErrorText = styled.span`
    color: ${({ theme }) => theme.color.secondary};
    font-size: 10px;
`

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
    style?: CSSProperties
    width?: string
    id: string
    errorText?: string
    placeholder?: string
    label?: string
    value: string
}

function Input({
    onChange,
    style = {},
    id = '',
    placeholder = '',
    label = '',
    errorText = '',
    width = '100%',
    value,
    ...rest
}: InputProps): JSX.Element {
    const hasError = useMemo(() => !!errorText, [errorText])

    return (
        <SInputWrapper wrapperWidth={width}>
            {label ? <SInputLabel htmlFor={id}>{label}</SInputLabel> : null}
            <SInput
                {...rest}
                id={id}
                placeholder={placeholder}
                style={style}
                value={value}
                hasError={hasError}
                onChange={onChange}
            />
            {hasError ? <ErrorText>{errorText}</ErrorText> : null}
        </SInputWrapper>
    )
}

export default Input
