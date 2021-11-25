import { useRedwood } from '@redwood.dev/client/react'
import { Emoji } from 'emoji-mart'
import data from 'emoji-mart/data/all.json'
import moment from 'moment'
import React, { useCallback } from 'react'
import styled from 'styled-components'
import Attachment from '../Attachment'
import useAddressBook from '../hooks/useAddressBook'
import useModal from '../hooks/useModal'
import useNavigation from '../hooks/useNavigation'
import useUsers from '../hooks/useUsers'
import NormalizeMessage from './NormalizeMessage'
import UserAvatar from './UserAvatar'

// import strToColor from '../utils/strToColor'

const MessageWrapper = styled.div`
    display: flex;
    padding: ${(props) => (props.firstByUser ? '20px 0 0' : '0')};
    border-radius: 8px;
    transition: 50ms all ease-in-out;
    // &:hover {
    //   background: ${(props) => props.theme.color.grey[300]};
    // }
`

const MessageSender = styled.div`
    font-weight: 500;
`

const SMessageTimestamp = styled.span`
    font-size: 10px;
    font-weight: 300;
    color: rgba(255, 255, 255, 0.4);
    margin-left: 4px;
`

function MessageTimestamp({ dayDisplay, displayTime }) {
    return (
        <SMessageTimestamp>
            {dayDisplay} {displayTime}
        </SMessageTimestamp>
    )
}

function getTimestampDisplay(timestamp) {
    const momentDate = moment.unix(timestamp)
    let dayDisplay = momentDate.format('MM/DD')
    const displayTime = momentDate.format('h:mm A')

    if (
        momentDate.format('MM/DD') === moment.unix(Date.now()).format('MM/DD')
    ) {
        dayDisplay = 'Today'
    } else if (
        momentDate.subtract(1, 'day') ===
        moment.unix(Date.now()).subtract(1, 'day')
    ) {
        dayDisplay = 'Yesterday'
    }
    return {
        dayDisplay,
        displayTime,
    }
}

const SUserAvatar = styled(UserAvatar)`
    cursor: pointer;
`

const SAttachment = styled(Attachment)`
    max-width: 500px;
`

function Message({ msg, isOwnMessage, onClickAttachment, messageIndex }) {
    const { selectedServer, selectedStateURI } = useNavigation()
    const { users, usersStateURI } = useUsers(selectedStateURI)
    const addressBook = useAddressBook()
    const userAddress = msg.sender.toLowerCase()
    const user = (users && users[userAddress]) || {}
    const displayName = addressBook[userAddress] || user.username || msg.sender
    const { dayDisplay, displayTime } = getTimestampDisplay(msg.timestamp)
    const { onPresent: onPresentContactsModal } = useModal('contacts')
    const { onPresent: onPresentUserProfileModal } = useModal('user profile')
    const { httpHost } = useRedwood()

    const showContactsModal = useCallback(() => {
        if (isOwnMessage) {
            onPresentUserProfileModal()
        } else {
            onPresentContactsModal({ initiallyFocusedContact: msg.sender })
        }
    }, [
        onPresentContactsModal,
        onPresentUserProfileModal,
        msg,
        msg && msg.sender,
        isOwnMessage,
    ])

    return (
        <MessageWrapper
            firstByUser={msg.firstByUser}
            key={selectedStateURI + messageIndex}
        >
            {msg.firstByUser ? (
                <SUserAvatar
                    address={userAddress}
                    onClick={showContactsModal}
                />
            ) : (
                <UserAvatarPlaceholder />
            )}
            <MessageDetails>
                {msg.firstByUser && (
                    <MessageSender>
                        {displayName}{' '}
                        <MessageTimestamp
                            dayDisplay={dayDisplay}
                            displayTime={displayTime}
                        />
                    </MessageSender>
                )}

                {/* <MessageText>{msg.text}</MessageText> */}
                {/* <MessageParse msgText={msg.text} /> */}
                <NormalizeMessage msgText={msg.text} />
                {(msg.attachments || []).map((attachment, j) => (
                    <SAttachment
                        key={`${selectedStateURI}${messageIndex},${j}`}
                        attachment={attachment}
                        url={`${httpHost}/messages[${messageIndex}]/attachments[${j}]?state_uri=${encodeURIComponent(
                            selectedStateURI,
                        )}`}
                        onClick={onClickAttachment}
                    />
                ))}
            </MessageDetails>
        </MessageWrapper>
    )
}

function MessageParse({ msgText }) {
    const colons = `:[a-zA-Z0-9-_+]+:`
    const skin = `:skin-tone-[2-6]:`
    const colonsRegex = new RegExp(`(${colons}${skin}|${colons})`, 'g')

    const msgBlock = msgText
        .split(colonsRegex)
        .filter((block) => !!block)
        .map((block, idx) => {
            if (data.emojis[block.replace(':', '').replace(':', '')]) {
                if (block[0] === ':' && block[block.length - 1] === ':') {
                    return <Emoji key={idx} emoji={block} size={21} />
                }
            }

            return block
        })

    return <SMessageParseContainer>{msgBlock}</SMessageParseContainer>
}

const SMessageParseContainer = styled.div`
    white-space: pre-wrap;
    span.emoji-mart-emoji {
        top: 4px;
    }
`
