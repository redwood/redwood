import 'emoji-mart/css/emoji-mart.css'
import React, {
    useState,
    useCallback,
    useRef,
    useEffect,
    Fragment,
} from 'react'
import styled, { useTheme } from 'styled-components'
import { IconButton, Tooltip } from '@material-ui/core'
import {
    SendRounded as SendIcon,
    AddCircleRounded as AddIcon,
} from '@material-ui/icons'
import * as tinycolor from 'tinycolor2'
import filesize from 'filesize.js'
import moment from 'moment'
import CloseIcon from '@material-ui/icons/Close'
import EmojiEmotionsIcon from '@material-ui/icons/EmojiEmotions'
import { Picker, Emoji } from 'emoji-mart'
import data from 'emoji-mart/data/all.json'
import { Node, createEditor, Editor, Transforms, Range } from 'slate'
import { withReact, ReactEditor } from 'slate-react'
import { withHistory } from 'slate-history'
import { useRedwood, useStateTree } from '@redwood.dev/client/react'

import Button from '../Button'
import Input from '../Input'
import Embed from '../Embed'
import EmojiQuickSearch from '../EmojiQuickSearch'
import TextBox from '../TextBox'
import NormalizeMessage from '../Chat/NormalizeMessage'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import UserAvatar from '../UserAvatar'
import useModal from '../../hooks/useModal'
import useServerRegistry from '../../hooks/useServerRegistry'
import useAPI from '../../hooks/useAPI'
import { isImage } from '../../utils/contentTypes'
import useNavigation from '../../hooks/useNavigation'
import useAddressBook from '../../hooks/useAddressBook'
import useUsers from '../../hooks/useUsers'
import emojiSheet from '../../assets/emoji-mart-twitter-images.png'
import downloadIcon from '../../assets/download.svg'
import cancelIcon from '../../assets/cancel-2.svg'
import uploadIcon from '../../assets/upload.svg'
import fileIcon from '../Attachment/file.svg'

// Styled Components Start
const Container = styled.div`
    display: flex;
    flex-direction: column;
    // height: 100%;
    flex-grow: 1;
    background-color: ${(props) => props.theme.color.grey[200]};
`

// Styled Components End

function Chat({ className }) {
    return <Container className={className}>Chat</Container>
}

export default Chat
