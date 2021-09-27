import React, { useContext, useEffect } from 'react'
import ReactDOM from 'react-dom'
import styled, { keyframes } from 'styled-components'
import * as tinycolor from 'tinycolor2'
import CloseIcon from '@material-ui/icons/Close'

import { Context } from '../../contexts/Modals'
import useModal from '../../hooks/useModal'
import Spacer from '../Spacer'

function Modal({
    children,
    height,
    modalKey,
    onOpen,
    closeModal: paramCloseModal,
}) {
    const modalRoot = document.getElementById('modal-root')
    const { activeModalKey } = useContext(Context)
    const { onDismiss } = useModal(modalKey)
    let closeModal = paramCloseModal

    useEffect(() => {
        if (!modalRoot) {
            return
        }
        if (modalKey !== activeModalKey) {
            return
        }
        if (onOpen) {
            onOpen()
        }
    }, [onOpen, modalRoot, activeModalKey, modalKey])

    closeModal = closeModal || onDismiss
    if (!modalRoot) {
        return null
    }
    if (modalKey !== activeModalKey) {
        return null
    }

    return ReactDOM.createPortal(
        <StyledModalWrapper>
            <StyledModalBackdrop onClick={closeModal} />
            <StyledResponsiveWrapper>
                <StyledModal height={height}>{children}</StyledModal>
            </StyledResponsiveWrapper>
        </StyledModalWrapper>,
        modalRoot,
    )
}

const StyledModalWrapper = styled.div`
    align-items: center;
    display: flex;
    flex-direction: column;
    justify-content: center;
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    left: 0;
`

const StyledModalBackdrop = styled.div`
    background-color: ${(props) =>
        tinycolor(props.theme.color.grey[600]).setAlpha(0.8)};
    position: absolute;
    top: 0;
    right: 0;
    bottom: 0;
    left: 0;
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
    @media (max-width: ${(props) => props.theme.breakpoints.mobile}px) {
        flex: 1;
        position: absolute;
        top: 100%;
        right: 0;
        left: 0;
        max-height: calc(100% - ${(props) => props.theme.spacing[4]}px);
        animation: ${mobileKeyframes} 0.3s forwards ease-out;
    }
`

const StyledModal = styled.div`
    height: ${(props) => props.height || 'unset'};
    background-color: ${(props) => props.theme.color.grey[400]};
    color: ${(props) => props.theme.color.white};
    border-radius: 6px;
    display: flex;
    flex-direction: column;
    position: relative;
    width: 100%;
    min-height: 0;
`

function ModalTitle({ children, closeModal }) {
    return (
        <StyledModalTitle>
            {children}
            <StyledModalClose onClick={closeModal}>
                <CloseIcon style={{ color: 'white' }} />
            </StyledModalClose>
        </StyledModalTitle>
    )
}

const StyledModalClose = styled.div`
    height: 100%;
    width: ${(props) => props.theme.topBarSize}px;
    background-color: ${(props) => props.theme.color.grey[200]};
    display: flex;
    align-items: center;
    justify-content: center;
    border-top-right-radius: 6px;
    cursor: pointer;
    transition: all ease-in-out 0.15s;
    &:hover {
        background-color: ${(props) => props.theme.color.grey[100]};
        svg {
            transform: scale(1.2);
            transition: all ease-in-out 0.15s;
        }
    }
`

const StyledModalTitle = styled.div`
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 20px;
    font-weight: 500;
    padding-left: 24px;
    height: ${(props) => props.theme.topBarSize}px;
    border-top-left-radius: 6px;
    border-top-right-radius: 6px;
    border-bottom: 1px solid ${(props) => props.theme.color.grey[200]};
`

function ModalContent({ className, children }) {
    return (
        <StyledModalContent className={className}>
            {children}
        </StyledModalContent>
    )
}

const StyledModalContent = styled.div`
    display: flex;
    flex-direction: column;
    padding: ${(props) => props.theme.spacing[4]}px;
    @media (max-width: ${(props) => props.theme.breakpoints.mobile}px) {
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
                    <StyledModalAction>{child}</StyledModalAction>
                    {i < l - 1 && <Spacer />}
                </>
            ))}
        </StyledModalActions>
    )
}

const StyledModalActions = styled.div`
    align-items: center;
    background-color: ${(props) => props.theme.color.grey[200]}00;
    display: flex;
    justify-content: space-evenly;
    margin: 0;
    padding: ${(props) => props.theme.spacing[3]}px;
`

const StyledModalAction = styled.div`
    // flex: 1;
`

export default Modal
export { ModalTitle, ModalContent, ModalActions }
