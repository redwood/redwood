import { Redirect } from 'react-router-dom'
import styled from 'styled-components'

import Card from '../UI/Card'
import Header from '../UI/Text/Header'
import P from '../UI/Text/P'
import ErrorOutput from '../UI/Text/ErrorOutput'
import Button from '../UI/Button'
import LoadingSection from '../UI/Loading/LoadingSection'
import hushLogo from '../../assets/images/hush-logo.svg'

const SConnection = styled.div`
    background: ${({ theme }) => theme.color.textDark};
    width: 100vw;
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
`

const SHeader = styled(Header)`
    font-size: 28px !important;
    text-align: center;
    color: ${({ theme }) => theme.color.secondary};
    border-bottom: 1px solid rgba(255, 255, 255, 0.12);
    padding-bottom: 12px;
`

const SCard = styled(Card)`
    padding: 8px 12px;
    box-sizing: border-box;
    height: auto;
    padding-bottom: 16px;
    position: relative;
`

const SButton = styled(Button)`
    font-weight: bold;
    width: 100%;
    margin-top: 24px !important;
`

const SP = styled(P)`
    font-size: 14px;
`

const SImage = styled.img`
    height: 48px;
`

const SImageWrapper = styled.div`
    display: flex;
    width: 100%;
    align-items: center;
    justify-content: center;
`

function Connection({
    checkLogin,
    isLoggedIn,
    connectionError,
    checkingLogin,
}) {
    if (!connectionError && isLoggedIn) {
        return <Redirect to="/" />
    }
    if (!connectionError && !isLoggedIn) {
        return <Redirect to="/profiles" />
    }

    return (
        <SConnection>
            <SCard>
                <SHeader>Node Connection Error</SHeader>
                <SImageWrapper>
                    <SImage src={hushLogo} alt="Hush Logo" />
                </SImageWrapper>
                <SP>
                    There was problem connecting to your local node. Please
                    restart the application and try again. If this problem
                    continues, please file a bug report.
                </SP>
                <ErrorOutput>Error Output: {connectionError}</ErrorOutput>
                <SButton
                    sType="primary"
                    onClick={checkLogin}
                    label="CLICK TO RETRY CONNECTION"
                />
                <LoadingSection
                    isLoading={checkingLogin}
                    loadingText="Checking node status..."
                />
            </SCard>
        </SConnection>
    )
}

export default Connection
