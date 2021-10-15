import { CSSProperties } from 'react'
import styled from 'styled-components'

interface MultiSelectProps {
    style?: CSSProperties
}

const SMultiSelect = styled.div`
    color: white;
`

function MultiSelect({ style = {} }: MultiSelectProps): JSX.Element {
    return <SMultiSelect style={style}>MultiSelect</SMultiSelect>
}

export default MultiSelect
