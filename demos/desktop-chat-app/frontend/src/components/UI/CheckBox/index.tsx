import { CSSProperties, SyntheticEvent } from 'react'
import styled, { css } from 'styled-components'
import {
    Checkbox,
    CheckboxProps as MUICheckboxProps,
    FormControlLabel,
} from '@material-ui/core'

interface CheckBoxProps extends MUICheckboxProps {
    style?: CSSProperties
    isChecked: boolean
    sType: string
    label?: string
    labelPlacement?: 'start' | 'end' | 'bottom' | 'top' | undefined
    onValueChanged?: (value: boolean, event: SyntheticEvent) => void
}

const cssPrimary = css`
    .MuiSvgIcon-root {
        color: ${({ theme }) => theme.color.primary};
    }
    .MuiTouchRipple-child {
        background-color: ${({ theme }) => theme.color.primary};
    }
`

const cssSecondary = css`
    .MuiSvgIcon-root {
        color: ${({ theme }) => theme.color.accent2};
    }
    .MuiTouchRipple-child {
        background-color: ${({ theme }) => theme.color.accent2};
    }
`
const cssDisabled = css`
    .MuiSvgIcon-root {
        color: ${({ theme }) => theme.color.accent1};
    }
    .MuiTouchRipple-child {
        background-color: ${({ theme }) => theme.color.accent1};
    }
    cursor: not-allowed;
    pointer-events: auto;
`

const SCheckBox = styled(Checkbox)<CheckBoxProps>`
    &&& {
        ${({ sType }) => sType === 'primary' && cssPrimary}
        ${({ sType }) => sType === 'secondary' && cssSecondary}
        ${({ disabled }) => disabled && cssDisabled}
    }
`

const SFormControlLabel = styled(FormControlLabel)`
    &&& {
        color: ${({ theme }) => theme.color.accent2};
        .MuiFormControlLabel-label {
            font-size: ${({ theme }) => `${theme.font.size.s2}px`};
            font-family: ${({ theme }) => theme.font.type.primary};
        }
    }
`

function CheckBox({
    style = {},
    isChecked = false,
    sType = 'primary',
    onValueChanged = () => true,
    label = '',
    labelPlacement,
    ...rest
}: CheckBoxProps): JSX.Element {
    const control = (
        <SCheckBox
            {...rest}
            onChange={(event) => onValueChanged(!isChecked, event)}
            style={style}
            sType={sType}
            isChecked={isChecked}
        />
    )

    if (label) {
        return (
            <SFormControlLabel
                labelPlacement={labelPlacement || 'start'}
                label={label}
                control={control}
            />
        )
    }

    return control
}

export default CheckBox
