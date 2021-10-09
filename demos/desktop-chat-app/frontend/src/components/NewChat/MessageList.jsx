import { useRef, useMemo, useState, useCallback, useEffect } from 'react'
import styled from 'styled-components'
import Message from './Message'
import AttachmentPreviewModal from '../Modal/AttachmentPreviewModal'
import useModal from '../../hooks/useModal'

const MessageContainer = styled.div`
    display: flex;
    flex-direction: column;
    flex-grow: 1;

    overflow-y: scroll;

    /* Chrome, Safari, Opera */
    &::-webkit-scrollbar {
        display: none;
    }
    -ms-overflow-style: none; /* IE and Edge */
    scrollbar-width: none; /* Firefox */
`
/* eslint-disable */
const MessageList = ({ messages = [], nodeIdentities }) => {
    const messageTextContainer = useRef()

    const { onPresent: onPresentPreviewModal } = useModal('attachment preview')
    const [previewedAttachment, setPreviewedAttachment] = useState({})
    const onClickAttachment = useCallback(
        (attachment, url) => {
            setPreviewedAttachment({ attachment, url })
            onPresentPreviewModal()
        },
        [setPreviewedAttachment, onPresentPreviewModal],
    )

    useEffect(() => {
        // Scrolls on new messages
        if (messageTextContainer.current) {
            setTimeout(() => {
                if (messageTextContainer.current !== null) {
                    messageTextContainer.current.scrollTop =
                        messageTextContainer.current.scrollHeight
                }
            }, 0)
        }
    }, [messages.length])

    const ownAddress = useMemo(
        () =>
            nodeIdentities && nodeIdentities[0]
                ? nodeIdentities[0].address
                : null,
        [nodeIdentities],
    )

    return (
        <>
            <MessageContainer ref={messageTextContainer}>
                {messages.map((msg, i) => (
                    <Message
                        msg={msg}
                        ownAddress={ownAddress}
                        onClickAttachment={onClickAttachment}
                        messageIndex={i}
                        key={msg.mapId}
                    />
                ))}
            </MessageContainer>

            <AttachmentPreviewModal
                attachment={previewedAttachment.attachment}
                url={previewedAttachment.url}
            />
        </>
    )
}
/* eslint-enable */
export default MessageList
