import { CSSProperties } from 'react'
import Fade from '@material-ui/core/Fade'
import styled from 'styled-components'
import loadingGoo from '../../Account/assets/loading-goo.svg'

interface LoadingSectionProps {
    className?: string
    style?: CSSProperties
    loadingText?: string
    isLoading: boolean
}

const SLoadingSection = styled.div`
    position: absolute;
    top: 0;
    left: 0;
    height: 100%;
    width: 100%;
    background: rgba(0, 0, 0, 0.85);
`

const SLoadingContent = styled.div`
    height: 100%;
    width: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    img {
        height: 64px;
        transform: scale(2.5);
    }
    span {
        color: ${({ theme }) => theme.color.text};
        font-size: 20px;
        margin-top: 16px;
        font-family: ${({ theme }) => theme.font.type.secondary};
    }
`

function LoadingSection({
    className = '',
    style = {},
    loadingText = 'Loading...',
    isLoading = false,
}: LoadingSectionProps): JSX.Element {
    return (
        <Fade in={isLoading} timeout={{ enter: 0, exit: 400 }}>
            <SLoadingSection style={style} className={className}>
                <SLoadingContent>
                    <img src={loadingGoo} alt="Loading" />
                    <span>{loadingText}</span>
                </SLoadingContent>
            </SLoadingSection>
        </Fade>
    )
}

export default LoadingSection
