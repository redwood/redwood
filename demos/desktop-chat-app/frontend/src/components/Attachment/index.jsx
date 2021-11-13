import React, { useState, useCallback, useRef, useEffect } from 'react'
import styled, { useTheme } from 'styled-components'
import filesize from 'filesize.js'
import Embed from '../Embed'
import { isImage, isPDF } from '../../utils/contentTypes'
import fileIcon from './file.svg'
import Button from './../Button'

const ImageWrapper = styled.div`
    padding: 4px 0;
    cursor: default;
`

const EmbedWrapper = styled.div`
    padding: 4px 4px;
    background-color: ${props => props.theme.color.grey[500]};
    border-radius: 8px;
    cursor: default;
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

const SDownloadLink = styled.a`
    color: ${props => props.theme.color.white};
	text-decoration: none;
	cursor: pointer;

    &:hover {
        color: hsl(231deg 36% 53%);
        text-decoration: underline;
    }
`

const SInvalidAttachmentWrapper = styled.div`
	display: flex;
	flex-direction: column;
	align-items: flex-start;
	> img {
		height: 120px;
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

function Attachment(props) {
	const { attachment, url, onClick, className } = props
	const [loadFailed, setLoadFailed] = useState(false)
    const _onClick = useCallback(() => {
        onClick(attachment, url)
    }, [attachment, url, onClick])

	let Wrapper
    if (isImage(attachment['Content-Type'])) {
        Wrapper = ImageWrapper
    } else if (isPDF(attachment['Content-Type'])) {
        Wrapper = EmbedWrapper
	}
	
	let whichClick = _onClick

	if (loadFailed === 'failed') {
		whichClick = () => { }
	}

	if (!Wrapper) {
		return (
			<SInvalidAttachmentWrapper>
				<Metadata>
					<Filename><SDownloadLink onClick={() => downloadImage(url, attachment.filename)}>{attachment.filename}</SDownloadLink> </Filename>
					<Filesize>({filesize(attachment['Content-Length'])})</Filesize>
				</Metadata>
				<img alt={'File'} src={fileIcon} />
				<Button
					primary
					style={{
						width: '120px',
						marginTop: 12,
						fontSize: 10,
						padding: '3px 6px',
						lineHeight: '1.25',
					}}
					onClick={() => downloadImage(url, attachment.filename)}
				>Download File ({filesize(attachment['Content-Length'])})</Button>
			</SInvalidAttachmentWrapper>)
	}

    return (
        <Wrapper loadFailed={loadFailed} className={className}>
            <Metadata>
                <Filename><SDownloadLink onClick={() => downloadImage(url, attachment.filename)}>{attachment.filename}</SDownloadLink> </Filename>
                <Filesize>({filesize(attachment['Content-Length'])})</Filesize>
            </Metadata>
            <Embed onClick={whichClick} loadFailed={loadFailed} setLoadFailed={setLoadFailed} contentType={attachment['Content-Type']} url={url} height={'150px'} />
        </Wrapper>
    )
}

function DownloadLink({ href, target, children, onClick }) {
    function _onClick(e) {
		e.stopPropagation()
		onClick()
    }
    return (
        <a onClick={_onClick}>{children}</a>
    )
}

export default Attachment
