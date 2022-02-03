import React, { useState, useCallback, useRef, useEffect } from 'react'
import styled from 'styled-components'
import { toast } from 'react-toastify'

import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Input, { InputLabel } from '../Input'
import Button from '../Button'
import UserAvatar from '../UserAvatar'
import { useRedwood, useStateTree } from '@redwood.dev/client/react'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'
import useNavigation from '../../hooks/useNavigation'
import useUsers from '../../hooks/useUsers'
import useServerAndRoomInfo from '../../hooks/useServerAndRoomInfo'
import UploadAvatar from '../UploadAvatar'
import cancelIcon from './assets/cancel.svg'

const SUserControlContainer = styled.div`
    display: flex;
    align-items: center;
    height: 56px;
    width: 100%;
    background-color: ${props => props.theme.color.grey[500]};
`

const SUserLeft = styled.div`
    width: calc(250px - 12px);
    display: flex;
    align-items: center;
    padding-left: 12px;
    transition: .15s ease-in-out all;
    height: 100%;
    ${props => !props.disabled && 'cursor: pointer;'}

    &:hover {
        background: ${props => props.theme.color.grey[300]};
    }
`

const UsernameWrapper = styled.div`
    width: 100px;
    display: flex;
    flex-direction: column;
    margin-left: 8px;
`

const Username = styled.div`
    overflow-x: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: white;
    font-weight: 700;
    font-size: 0.8rem;
`

const NodeAddress = styled.div`
    overflow-x: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 10px;
    color: rgba(255, 255, 255, .6);
    font-weight: 300;
`

const SUserAvatar = styled(UserAvatar)`
    height: 40px;
`

const SCloseBtnContainer = styled.div`
	height: 16px;
	width: 16px;
	cursor: pointer;
	img {
		height: 14px;
		width: 12px;
	}
`

const ToastCloseBtn = ({ closeToast }) => (
	<SCloseBtnContainer className="Toastify__close-button Toastify__close-button--light" onClick={closeToast}>
		<img src={cancelIcon} alt="Cancel Icon" />
	</SCloseBtnContainer>
)

function UserControl() {
    let { onPresent, onDismiss } = useModal('user profile')
    let { httpHost, nodeIdentities } = useRedwood()
    let { selectedStateURI } = useNavigation()
    let { users, usersStateURI } = useUsers(selectedStateURI)
    let [username, setUsername] = useState(null)
    let [userPhotoURL, setUserPhotoURL] = useState(null)
    let nodeAddress = !!nodeIdentities && nodeIdentities.length > 0 ? nodeIdentities[0].address : null

    useEffect(() => {
        if (users && users[nodeAddress]) {
            setUsername(users[nodeAddress].username)
            if (users[nodeAddress].photo) {
                setUserPhotoURL(`${httpHost}/users/${nodeAddress}/photo?state_uri=${usersStateURI}&${Date.now()}`)
            } else {
                setUserPhotoURL(null)
            }
        } else {
            setUsername(null)
            setUserPhotoURL(null)
        }
    }, [users, httpHost, nodeAddress, usersStateURI])

    return (
        <SUserControlContainer>
            <SUserLeft disabled={!selectedStateURI} onClick={!!selectedStateURI ? onPresent : null}>
            <SUserAvatar address={nodeAddress} />
                <UsernameWrapper>
                    <Username>{!!username ? username : nodeAddress}</Username>
                    <NodeAddress>{!!username ? nodeAddress : null}</NodeAddress>
                </UsernameWrapper>
            </SUserLeft>
            <UserProfileModal
                onDismiss={onDismiss}
                currentUsername={username}
				userPhotoURL={userPhotoURL}
				nodeAddress={nodeAddress}
            />
        </SUserControlContainer>
    )
}

const SInput = styled(Input)`
	min-width: 280px;
`

const SToastContent = styled.div`
	background: #2a2d32;
	color: rgba(255, 255, 255, .8);
	font-size: 16px;
`

function UserProfileModal({ onDismiss, currentUsername, userPhotoURL, nodeAddress }) {
    const [username, setUsername] = useState('')
    const [iconImg, setIconImg] = useState(null)
    const [iconFile, setIconFile] = useState(null)
    const { nodeIdentities } = useRedwood()
    const api = useAPI()
    const { selectedStateURI } = useNavigation()
    const { usersStateURI } = useUsers(selectedStateURI)
    const photoFileRef = useRef()

    useEffect(() => {
      if (currentUsername) {
        setUsername(currentUsername)
        setIconImg(userPhotoURL)
      }
	}, [currentUsername, userPhotoURL])
	
	const copyPublicKey = () => {
		navigator.clipboard.writeText(nodeAddress)
		toast(<SToastContent>Public Key Copied!</SToastContent>, {
			autoClose: 4500,
			style: {
				background: '#2a2d32',
			},
			closeButton: ToastCloseBtn,
		})
	}

    const onSave = useCallback(async () => {
        if (!api || !nodeIdentities || nodeIdentities.length === 0) { return }
        try {
            // let photoFile
            // if (photoFileRef && photoFileRef.current && photoFileRef.current.files && photoFileRef.current.files.length > 0) {
            //     photoFile = photoFileRef.current.files[0]
            // }
            await api.updateProfile(nodeIdentities[0].address, usersStateURI, username, iconFile)
            onDismiss()
        } catch (err) {
            console.error(err)
        }
    }, [api, nodeIdentities, usersStateURI, username, iconFile, onDismiss])

    const onChangeUsername = useCallback((e) => {
        if (e.code === 'Enter') {
            onSave()
        } else {
            setUsername(e.target.value)
        }
    }, [onSave, setUsername])

    function closeModal() {
      setUsername(currentUsername)
      onDismiss()
    }

    return (
        <Modal modalKey="user profile">
            <ModalTitle closeModal={closeModal}>Your Profile</ModalTitle>
            <ModalContent>
                {/* <div>
                    <input type="file" ref={photoFileRef} />
                </div> */}
                <UploadAvatar
                  iconImg={iconImg}
                  setIconImg={setIconImg}
                  setIconFile={setIconFile}
                />
				<InputLabel label={'Username'}>
					<SInput
						value={username}
						onChange={onChangeUsername}
                        onEnter={onSave}
                        autoFocus
					/>
				</InputLabel>
                {/* <div>
                    Username:
                    <Input value={username} onChange={onChangeUsername} />
                </div> */}
            </ModalContent>
            <ModalActions>
				<Button onClick={copyPublicKey}>Copy Key</Button>
                <Button onClick={onSave} primary>Save</Button>
            </ModalActions>
        </Modal>
    )
}

export default UserControl