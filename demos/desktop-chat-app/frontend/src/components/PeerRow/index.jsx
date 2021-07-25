import React, { useState, useCallback, useEffect, useRef } from 'react'
import styled, { useTheme } from 'styled-components'
import { IconButton } from '@material-ui/core'
import { Check as CheckIcon } from '@material-ui/icons'
import moment from 'moment'

import Input from '../Input'
import UserAvatar from '../UserAvatar'
import { useRedwood, useStateTree } from '@redwood.dev/client/react'
import useAPI from '../../hooks/useAPI'
import usePeers from '../../hooks/usePeers'
import useUsers from '../../hooks/useUsers'
import useAddressBook from '../../hooks/useAddressBook'
import useNavigation from '../../hooks/useNavigation'
import theme from '../../theme'

const PeerNameContainer = styled.div`
    display: flex;
    align-items: center;
    border-radius: 8px;
    padding: 8px;
    user-select: none;
    ${props => props.onClick ? 'cursor: pointer;' : ''}
    transition: all 500ms ease-out;

    &.clicked {
        background-color: ${props => props.theme.color.grey[200]};
        transition: all 200ms cubic-bezier(0, 0.98, 0.45, 1);
    }
`

const PeerNameTitle = styled.h3`
    margin: 0;
    cursor: pointer;
    overflow-x: hidden;
    text-overflow: ellipsis;
    font-weight: ${props => props.$bold ? '700' : '400'};
`

const PeerNameSubtitle = styled.h4`
    // font-size: 0.7rem;
    // margin: 0 0 0 16px;
    margin: 0;
    color: ${props => props.theme.color.grey[100]};
    font-weight: 300;
`

const PeerLastSeen = styled.div`
    // margin-top: -4px;
    font-size: 0.8rem;
    color: ${props => props.theme.color.grey[100]};
`

const SUserAvatar = styled(UserAvatar)`
    margin-right: 12px;
`

const SInput = styled(Input)`
    width: unset;
    height: 13px;
`

const SCheckIcon = styled(CheckIcon)`
    color: ${props => props.theme.color.green[500]};
    cursor: pointer;
    margin-left: 6px;
`

const Wrapper = styled.div`
    display: flex;
    flexDirection: column;
    flexGrow: 1
`

const UsernameIfNickname = styled.span`
    margin-left: 12px;
    font-weight: 500;
    color: ${props => props.theme.color.grey[100]};
`

function PeerRow({ address, editable, showLastSeen, boldName, onClick }) {
    let { peersByAddress } = usePeers()
    let { selectedStateURI } = useNavigation()
    let { users } = useUsers(selectedStateURI)
    let addressBook = useAddressBook()
    let api = useAPI()
    let [isEditingNickname, setIsEditingNickname] = useState(false)
    let [newNickname, setNewNickname] = useState()
    let [clicked, setClicked] = useState(false)
    let theme = useTheme()

    useEffect(() => {
        if (!!addressBook[address]) {
            setNewNickname(addressBook[address])
        }
    }, [addressBook, address, setNewNickname])

    let onClickEditNickname = useCallback(() => {
        if (!editable) { return }
        setIsEditingNickname(true)
    }, [editable, setIsEditingNickname])

    let onClickSetNickname = useCallback(async () => {
        await api.setNickname(address, newNickname)
        setIsEditingNickname(false)
    }, [api, address, setIsEditingNickname, newNickname])

    let onChangeNewNickname = useCallback(evt => {
        setNewNickname(evt.target.value)
    }, [setNewNickname])

    let onKeyDownNewNickname = useCallback(evt => {
        if (evt.key === 'Enter') {
            evt.stopPropagation()
            onClickSetNickname()
        } else if (evt.key === 'Escape') {
            evt.stopPropagation()
            setIsEditingNickname(false)
            setNewNickname(addressBook[address])
        }
    }, [onClickSetNickname])

    let onMouseDown = useCallback(() => {
        setClicked(true)
    }, [setClicked])

    let onMouseUp = useCallback(() => {
        setClicked(false)
    }, [setClicked])

    if (!address) {
        return null
    }
    let peer = peersByAddress[address]
    if (!peer) {
        return null
    }
    let user = (users || {})[address] || {}
    return (
        <PeerNameContainer
            onClick={onClick}
            onMouseDown={onClick && onMouseDown}
            onMouseUp={onClick && onMouseUp}
            onMouseLeave={onClick && onMouseUp}
            className={clicked ? 'clicked' : null}
        >
            <SUserAvatar address={address} />

            <div style={{ display: 'flex', flexDirection: 'column', flexGrow: 1 }}>
                {!isEditingNickname &&
                    <PeerNameTitle
                        $bold={boldName}
                        onClick={onClickEditNickname}
                    >
                        {addressBook[peer.address] || user.username || peer.address}
                        {!!addressBook[peer.address] && user.username &&
                            <UsernameIfNickname>({user.username})</UsernameIfNickname>
                        }
                    </PeerNameTitle>
                }
                {isEditingNickname &&
                    <div style={{ display: 'flex' }}>
                        <SInput autoFocus value={newNickname} onChange={onChangeNewNickname} onKeyDown={onKeyDownNewNickname} />
                        <SCheckIcon onClick={onClickSetNickname} />
                    </div>
                }
                {!!addressBook[peer.address] &&
                    <PeerNameSubtitle>{peer.address}</PeerNameSubtitle>
                }
            </div>

            {showLastSeen &&
                <PeerLastSeen>Last seen {moment(peer.lastContact).fromNow()}</PeerLastSeen>
            }
        </PeerNameContainer>
    )
}

export default PeerRow