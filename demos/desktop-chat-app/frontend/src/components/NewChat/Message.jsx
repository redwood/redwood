import React, { useCallback, useMemo, memo } from 'react'
import styled from 'styled-components'
import moment from 'moment'
import { useRedwood } from '../redwood.js/dist/main/react'

import NormalizeMessage from '../Chat/NormalizeMessage'
import UserAvatar from '../UserAvatar'
import Attachment from '../Attachment'
import useNavigation from '../../hooks/useNavigation'
import useUsers from '../../hooks/useUsers'
import useAddressBook from '../../hooks/useAddressBook'
import useModal from '../../hooks/useModal'

const MessageWrapper = styled.div`
    display: flex;
    padding: ${(props) => (props.firstByUser ? '20px 0 0' : '0')};
    border-radius: 8px;
    transition: 50ms all ease-in-out;
    // &:hover {
    //   background: ${(props) => props.theme.color.grey[300]};
    // }
`

const SUserAvatar = styled(UserAvatar)`
    cursor: pointer;
`

const UserAvatarPlaceholder = styled.div`
    padding-left: 40px;
    // width: 40px;
`

const MessageDetails = styled.div`
    display: flex;
    flex-direction: column;
    padding-left: 14px;
    padding-bottom: 6px;
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

const SAttachment = styled(Attachment)`
    max-width: 500px;
`

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

function MessageTimestamp({ dayDisplay, displayTime }) {
    return (
        <SMessageTimestamp>
            {dayDisplay} {displayTime}
        </SMessageTimestamp>
    )
}

function Message({ msg = {}, onClickAttachment, ownAddress, messageIndex }) {
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

    const isOwnMessage = useMemo(
        () => msg.sender === ownAddress,
        [msg, ownAddress],
    )

    const showContactsModal = useCallback(() => {
        if (isOwnMessage) {
            onPresentUserProfileModal()
        } else {
            onPresentContactsModal({ initiallyFocusedContact: msg.sender })
        }
    }, [
        onPresentContactsModal,
        onPresentUserProfileModal,
        msg.sender,
        isOwnMessage,
    ])
    /* eslint-disable */
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
    /* eslint-enable */
}

export default memo(Message)
