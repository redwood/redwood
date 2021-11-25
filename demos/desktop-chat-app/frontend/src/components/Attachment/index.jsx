import React, { useState, useCallback, useRef, useEffect } from 'react'
import styled, { useTheme } from 'styled-components'
import filesize from 'filesize.js'
import VideoJS from './../VideoJS'
import Embed from '../Embed'
import { isImage, isPDF, isVideo, isAudio } from '../../utils/contentTypes'
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

const loadAudio = async (url) => {
	try {
		let resp = await fetch(url, {
            method: 'GET',
		})
		const blob = await resp.blob()
		const downloadUrl = window.URL.createObjectURL(blob)
		return downloadUrl
	} catch (err) {
		console.log('err', err)
	}
}

function Attachment(props) {
	const { attachment, url, onClick, className } = props

	const [loadFailed, setLoadFailed] = useState(false)
	const [isLoading, setIsLoading] = useState(true)
	const audioRef = useRef(null)
    const _onClick = useCallback(() => {
        onClick(attachment, url)
	}, [attachment, url, onClick])
	
	useEffect(async () => {
		if (url) {
			try {
				const downloadUrl = await loadAudio(url)
				setIsLoading(false)
				audioRef.current.src = downloadUrl
				console.log(audioRef)
			} catch (err) {
				console.log('err', err)
			}
		}
	}, [url])

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

	if (isAudio(attachment['Content-Type'])) {

		if (isLoading) {
			return <div>Loading audio...</div>
		}

		return (
			<audio
				ref={audioRef}
				controls
				src={url}
			>
				Your browser does not support the
				<code>audio</code> element.
			</audio>
		)
	}

	if (isVideo(attachment.filename)) {
		const videoJsOptions = {
			autoplay: false,
			playbackRates: [0.5, 1, 1.25, 1.5, 2],
			width: 720,
			height: 300,
			controls: true,
			sources: [
			  {
				src: url,
				type: "video/mp4",
			  },
			],
		  };

		//   return (
		// 	  <video>
		// 		  <source type="video/mp4" src="http://localhost:8080/messages[2]/attachments[0]?state_uri=test%2Ftest&filename=else.mp4" />
		// 	  </video>
		//   )

		  return (
			<div>
				  <VideoJS {...videoJsOptions} />
		  </div>
		  )
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
