import { CSSProperties } from 'react'
import styled from 'styled-components'
import CheckBox from '../CheckBox'

interface MultiSelectProps {
    style?: CSSProperties
    children: JSX.Element
    checked: boolean
    onValueChanged: () => void
    sType?: string
}

const SMultiSelect = styled.div<{ checked: boolean }>`
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;
    cursor: pointer;
    transition: ${({ theme }) => theme.transition.primary};
    border-radius: 2px;
    background: ${({ theme, checked }) =>
        checked ? `${theme.color.elevation2} !important` : 'transparent'};
    &:hover {
        background: ${({ theme }) => theme.color.elevation1};
    }
`

const SMultiSelectLeft = styled.div`
    display: flex;
    align-items: center;
    justify-content: center;
`

const SMultiSelectRight = styled.div`
    display: flex;
    align-items: center;
    justify-content: center;
`

function MultiSelect({
    style = {},
    children,
    checked = false,
    onValueChanged = () => false,
    sType = 'primary',
}: MultiSelectProps): JSX.Element {
    return (
        <SMultiSelect checked={checked} onClick={onValueChanged} style={style}>
            <SMultiSelectLeft>{children}</SMultiSelectLeft>
            <SMultiSelectRight>
                <CheckBox sType={sType} checked={checked} />
            </SMultiSelectRight>
        </SMultiSelect>
    )
}

export default MultiSelect
