import React, { useCallback } from 'react'
import styled from 'styled-components'

import Button from '../Button'
import { Pane, PaneContent, PaneActions } from '../SlidingPane'

import theme from '../../theme'
import linodeLogo from '../../assets/linode.png'

const HostingProviderContainer = styled.div`
    display: flex;
    justify-content: space-evenly;
`

const HostingProviderBox = styled.div`
    width: 150px;
    height: 150px;
    padding: 20px;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    border-radius: 12px;
    cursor: pointer;
    border: ${(props) =>
        props.highlighted ? `4px solid ${theme.color.indigo[500]}` : 'unset'};
    background-color: ${() => theme.color.grey[200]};
    transition: background-color 0.1s ease-in-out;

    &:hover {
        background-color: ${() => theme.color.grey[100]};

        a {
            color: ${theme.color.white};
        }
    }
`

const ProviderLogo = styled.img`
    width: 100%;
`

const PaneTitle = styled.div`
    font-size: 1.1rem;
    font-weight: 700;
    text-align: center;
    margin-bottom: 12px;
`

const PaneSubtitle = styled.div`
    margin-bottom: 12px;
`

const ProviderLink = styled.a`
    color: ${theme.color.indigo[500]};
    text-decoration: none;
    transition: all 0.1s ease-in-out;
    &:visited {
        color: ${theme.color.indigo[500]};
    }
    &:hover {
        text-decoration: underline;
    }
    &:active {
        color: ${theme.color.indigo[500]};
    }
`

function ChooseHostingPane({
    provider,
    setProvider,
    onClickBack,
    onClickNext,
    ...props
}) {
    const onClickNone = useCallback(() => {
        setProvider('none')
    }, [setProvider])

    const onClickLinode = useCallback(() => {
        setProvider('linode')
    }, [setProvider])

    const handleBack = useCallback(() => {
        setProvider('none')
        onClickBack()
    }, [onClickBack, setProvider])

    return (
        <Pane {...props}>
            <PaneContent>
                <PaneTitle>Do you need help hosting a public node?</PaneTitle>
                <PaneSubtitle>
                    Even though Redwood is fully peer-to-peer, it is usually
                    necessary to host a node on a publicly-accessible cloud so
                    that other users can find you. Advanced users who are
                    familiar with setting up infrastructure can skip this step.
                </PaneSubtitle>

                <HostingProviderContainer>
                    <HostingProviderBox
                        onClick={onClickNone}
                        highlighted={!provider || provider === 'none'}
                    >
                        I do not need hosting
                    </HostingProviderBox>

                    <HostingProviderBox
                        onClick={onClickLinode}
                        highlighted={provider === 'linode'}
                    >
                        <ProviderLogo src={linodeLogo} />
                        <ProviderLink href="https://linode.com" target="_blank">
                            linode.com
                        </ProviderLink>
                    </HostingProviderBox>
                </HostingProviderContainer>
            </PaneContent>

            <PaneActions>
                <Button onClick={handleBack}>Back</Button>
                <Button
                    onClick={onClickNext}
                    primary
                    disabled={(provider || '').trim().length === 0}
                >
                    Next
                </Button>
            </PaneActions>
        </Pane>
    )
}

export default ChooseHostingPane
