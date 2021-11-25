import React, { useState, useCallback, useEffect } from 'react'
import styled from 'styled-components'
import { Check as CheckIcon } from '@material-ui/icons'
import moment from 'moment'

import Input from '../Input'
import UserAvatar from '../UserAvatar'
import useAPI from '../../hooks/useAPI'
import usePeers from '../../hooks/usePeers'
import useUsers from '../../hooks/useUsers'
import useAddressBook from '../../hooks/useAddressBook'
import useNavigation from '../../hooks/useNavigation'

const PeerNameContainer = styled.div`
    display: flex;
    align-items: center;
    border-radius: 8px;
    padding: 8px;
    user-select: none;
    ${(props) => (props.onClick ? 'cursor: pointer;' : '')}
    transition: all 500ms ease-out;

    &.clicked {
        background-color: ${(props) => props.theme.color.grey[200]};
        transition: all 200ms cubic-bezier(0, 0.98, 0.45, 1);
    }
`

const PeerNameTitle = styled.h3`
    margin: 0;
    cursor: pointer;
    overflow-x: hidden;
    text-overflow: ellipsis;
    font-weight: ${(props) => (props.$bold ? '700' : '400')};
`

const PeerNameSubtitle = styled.h4`
    margin: 0;
    color: ${(props) => props.theme.color.grey[100]};
    font-weight: 300;
`

const PeerLastSeen = styled.div`
    font-size: 0.8rem;
    color: ${(props) => props.theme.color.grey[100]};
`

const SUserAvatar = styled(UserAvatar)`
    margin-right: 12px;
`

const SInput = styled(Input)`
    width: unset;
    height: 13px;
`

const SCheckIcon = styled(CheckIcon)`
    color: ${(props) => props.theme.color.green[500]};
    cursor: pointer;
    margin-left: 6px;
`

const UsernameIfNickname = styled.span`
    margin-left: 12px;
    font-weight: 500;
    color: ${(props) => props.theme.color.grey[100]};
`

const PeerOptionWrapper = styled.div`
    display: flex;
    flex-direction: column;
    flex-grow: 1;
`

function PeerRow({ address, editable, showLastSeen, boldName, onClick }) {
    const { peersByAddress } = usePeers()
    const { selectedStateURI } = useNavigation()
    const { users } = useUsers(selectedStateURI)
    const addressBook = useAddressBook()
    const api = useAPI()
    const [isEditingNickname, setIsEditingNickname] = useState(false)
    const [newNickname, setNewNickname] = useState()
    const [clicked, setClicked] = useState(false)

    useEffect(() => {
        if (addressBook[address]) {
            setNewNickname(addressBook[address])
        }
    }, [addressBook, address, setNewNickname])

    const onClickEditNickname = useCallback(() => {
        if (!editable) {
            return
        }
        setIsEditingNickname(true)
    }, [editable, setIsEditingNickname])

    const onClickSetNickname = useCallback(async () => {
        await api.setNickname(address, newNickname)
        setIsEditingNickname(false)
    }, [api, address, setIsEditingNickname, newNickname])

    const onChangeNewNickname = useCallback(
        (evt) => {
            setNewNickname(evt.target.value)
        },
        [setNewNickname],
    )

    const onKeyDownNewNickname = useCallback(
        (evt) => {
            if (evt.key === 'Enter') {
                evt.stopPropagation()
                onClickSetNickname()
            } else if (evt.key === 'Escape') {
                evt.stopPropagation()
                setIsEditingNickname(false)
                setNewNickname(addressBook[address])
            }
        },
        [onClickSetNickname],
    )

    const onMouseDown = useCallback(() => {
        setClicked(true)
    }, [setClicked])

    const onMouseUp = useCallback(() => {
        setClicked(false)
    }, [setClicked])

    if (!address) {
        return null
    }
    const peer = peersByAddress[address]
    if (!peer) {
        return null
    }
    const user = (users || {})[address] || {}
    return (
        <PeerNameContainer
            onClick={onClick}
            onMouseDown={onClick && onMouseDown}
            onMouseUp={onClick && onMouseUp}
            onMouseLeave={onClick && onMouseUp}
            className={clicked ? 'clicked' : null}
        >
            <SUserAvatar address={address} />

            <PeerOptionWrapper>
                {!isEditingNickname && (
                    <PeerNameTitle
                        $bold={boldName}
                        onClick={onClickEditNickname}
                    >
                        {addressBook[peer.address] ||
                            user.username ||
                            peer.address}
                        {!!addressBook[peer.address] && user.username && (
                            <UsernameIfNickname>
                                ({user.username})
                            </UsernameIfNickname>
                        )}
                    </PeerNameTitle>
                )}
                {isEditingNickname && (
                    <div style={{ display: 'flex' }}>
                        <SInput
                            autoFocus
                            value={newNickname}
                            onChange={onChangeNewNickname}
                            onKeyDown={onKeyDownNewNickname}
                        />
                        <SCheckIcon onClick={onClickSetNickname} />
                    </div>
                )}
                {!!addressBook[peer.address] && (
                    <PeerNameSubtitle>{peer.address}</PeerNameSubtitle>
                )}
            </PeerOptionWrapper>

            {showLastSeen && (
                <PeerLastSeen>
                    Last seen {moment(peer.lastContact).fromNow()}
                </PeerLastSeen>
            )}
        </PeerNameContainer>
    )
}

export default PeerRow
