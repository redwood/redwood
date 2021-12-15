import React, { useState, useCallback, useRef, useEffect, Fragment } from 'react'
import styled, { useTheme } from 'styled-components'
import filesize from 'filesize.js'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Embed from '../Embed'
import useModal from '../../hooks/useModal'

const SModalContent = styled(ModalContent)`
    width: 600px;
    flex-direction: column;
`

const Metadata = styled.div`
    padding-bottom: 4px;
`

const Filename = styled.span`
    font-size: 0.8rem;
`

const Filesize = styled.span`
    font-size: 0.8rem;
    color: ${props => props.theme.color.grey[100]};
`

function AttachmentModal() {
    let { activeModalProps: { attachment, url } } = useModal('attachment preview')
    if (!attachment) {
        return null
    }
    return (
        <Modal modalKey="attachment preview">
            <SModalContent>
                <Metadata>
                    <Filename>{attachment.filename} </Filename>
                    <Filesize>({filesize(attachment['Content-Length'])})</Filesize>
                </Metadata>
                <Embed contentType={attachment['Content-Type']} url={url} width={600} />
            </SModalContent>
        </Modal>
    )
}

export default AttachmentModal