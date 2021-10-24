import { CSSProperties } from 'react'
import styled from 'styled-components'

import hushLogo from '../../../assets/images/hush-logo.svg'

interface HushLogoProps {
    style?: CSSProperties
    className?: string
}

const SHushLogo = styled.img`
    display: block;
    height: 36px;
`

function HushLogo({ style = {}, className = '' }: HushLogoProps): JSX.Element {
    return (
        <SHushLogo
            style={style}
            className={className}
            src={hushLogo}
            alt="Hush Logo"
        />
    )
}

export default HushLogo
