import 'emoji-mart/css/emoji-mart.css'
import styled, { useTheme } from 'styled-components'
import filesize from 'filesize.js'
import React from 'react'
import cancelIcon from '../../assets/cancel-2.svg'
import downloadIcon from '../../assets/download.svg'
import useModal from '../../hooks/useModal'
import Embed from '../Embed'
import Modal from '../Modal'


const SModalContent = styled(ModalContent)`
    width: 600px;
    flex-direction: column;
    align-items: center;
    padding: 24px;
`

const Metadata = styled.div`
    padding-top: 8px;
`

const Filename = styled.span`
    font-size: 1rem;
`

const Filesize = styled.span`
    font-size: 1rem;
    color: ${props => props.theme.color.grey[100]};
`

const FileActionWrapper = styled.div`
  display: flex;
  justify-content: flex-end;
  transform: translateY(-12px);
  width: 100%;
  img {
      cursor: pointer;
      transition: all .1s ease-in-out;
      height: 28px;
      &:first-child {
          margin-right: 18px;
      }
      &:hover {
          transform: scale(1.1);
      }
  }
`

const downloadImage = async (url, fileName) => {
    const image = await fetch(url)
    const imageBlog = await image.blob()
    const imageURL = URL.createObjectURL(imageBlog)
    const link = document.createElement('a')
    link.href = imageURL
    link.download = fileName
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
}

function AttachmentPreviewModal({ attachment, url }) {
    const { onDismiss } = useModal('attachment preview')

    if (!attachment) {
        return null
    }
    return (
        <Modal modalKey="attachment preview">
            <SModalContent>
                <FileActionWrapper>
                    <img alt="Download" src={downloadIcon} onClick={() => downloadImage(url, attachment.filename)} />
                    <img alt="Close" src={cancelIcon}  onClick={onDismiss} />
                </FileActionWrapper>
                <Embed contentType={attachment['Content-Type']} url={url} height={'350px'} />
                <Metadata>
                    <Filename>{attachment.filename} </Filename>
                    <Filesize>({filesize(attachment['Content-Length'])})</Filesize>
                </Metadata>
            </SModalContent>
        </Modal>
    )
}
