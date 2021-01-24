import React, { useContext } from 'react'
import ReactDOM from 'react-dom'
import styled, { keyframes } from 'styled-components'
import * as tinycolor from 'tinycolor2'
import { Context } from '../../contexts/Modals'
import useModal from '../../hooks/useModal'
import Spacer from '../Spacer'


function Modal({ children, height, modalKey }) {
    const modalRoot = document.getElementById('modal-root')
    const { activeModalKey } = useContext(Context)
    const { onDismiss } = useModal(modalKey)
    if (!modalRoot) {
        return null
    } else if (modalKey !== activeModalKey) {
        return null
    }
    return ReactDOM.createPortal(
        <StyledModalWrapper>
            <StyledModalBackdrop onClick={onDismiss} />
                <StyledResponsiveWrapper>
                    <StyledModal height={height}>{children}</StyledModal>
                </StyledResponsiveWrapper>
        </StyledModalWrapper>
    , modalRoot)
}

const StyledModalWrapper = styled.div`
    align-items: center;
    display: flex;
    flex-direction: column;
    justify-content: center;
    position: fixed;
    top: 0; right: 0; bottom: 0; left: 0;
`

const StyledModalBackdrop = styled.div`
    background-color: ${props => tinycolor(props.theme.color.grey[600]).setAlpha(0.8)};
    position: absolute;
    top: 0; right: 0; bottom: 0; left: 0;
`

const mobileKeyframes = keyframes`
    0% {
        transform: translateY(0%);
    }
    100% {
        transform: translateY(-100%);
    }
`

const StyledResponsiveWrapper = styled.div`
    align-items: center;
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
    position: relative;
    width: 100%;
    max-width: 512px;
    @media (max-width: ${props => props.theme.breakpoints.mobile}px) {
        flex: 1;
        position: absolute;
        top: 100%;
        right: 0;
        left: 0;
        max-height: calc(100% - ${props => props.theme.spacing[4]}px);
        animation: ${mobileKeyframes} 0.3s forwards ease-out;
    }
`

const StyledModal = styled.div`
    height: ${props => props.height || 'unset'};
    padding: 0 20px;
    background-color: ${props => props.theme.color.grey[200]};
    color: ${props => props.theme.color.white};
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    position: relative;
    width: 100%;
    min-height: 0;
`

function ModalTitle({ children }) {
    return (
        <StyledModalTitle>
            {children}
        </StyledModalTitle>
    )
}

const StyledModalTitle = styled.div`
    align-items: center;
    display: flex;
    font-size: 20px;
    font-weight: 700;
    height: ${props => props.theme.topBarSize}px;
    justify-content: center;
`

function ModalContent({ children }) {
    return <StyledModalContent>{children}</StyledModalContent>
}

const StyledModalContent = styled.div`
    display: flex;
    padding: ${props => props.theme.spacing[4]}px;
    @media (max-width: ${props => props.theme.breakpoints.mobile}px) {
        flex: 1;
        overflow: auto;
    }
`

function ModalActions({ children }) {
    const l = React.Children.toArray(children).length
    return (
        <StyledModalActions>
            {React.Children.map(children, (child, i) => (
                <>
                    <StyledModalAction>
                        {child}
                    </StyledModalAction>
                    {i < l - 1 && <Spacer />}
                </>
            ))}
        </StyledModalActions>
    )
}

const StyledModalActions = styled.div`
    align-items: center;
    background-color: ${props => props.theme.color.grey[200]}00;
    display: flex;
    justify-content: space-evenly;
    margin: 0;
    padding: ${props => props.theme.spacing[4]}px;
`

const StyledModalAction = styled.div`
    // flex: 1;
`

export default Modal
export {
    ModalTitle,
    ModalContent,
    ModalActions,
}
