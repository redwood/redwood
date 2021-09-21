import React, { forwardRef, useState } from 'react'
import styled from 'styled-components'
import filesize from 'filesize.js'
import ReactAudioPlayer from 'react-audio-player'

import Button from './../Button'

const loadFile = async (url) => {
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

const SAudioPlayer = styled.div`
	margin-top: 12px;
	display: flex;
	flex-direction: column;
`

const AudioTitleWrapper = styled.div`
	display: flex;
	background: #1c1e21;
	justify-content: space-between;
	padding-left: 8px;
	padding-right: 8px;
	padding-top: 4px;
	padding-bottom: 8px;
	border: 1px solid rgba(255, 255, 255, .2);
	span {
		font-size: 10px;
		color: rgba(255, 255, 255, .8);
		&:nth-child(2) {
			color: rgba(255, 255, 255, .4);
		}
	}
`

function AudioPlayer(props, ref) {
	const [didDownload, setDidDownload] = useState(false)
	const [isLoading, setIsLoading] = useState(false)
	const [isDownloading, setIsDownloading] = useState(false)

	const downloadImage = async (url, fileName) => {
		setIsDownloading(true)
		const image = await fetch(url)
		const imageBlog = await image.blob()
		const imageURL = URL.createObjectURL(imageBlog)
		const link = document.createElement('a')
		link.href = imageURL
		link.download = fileName
		document.body.appendChild(link)
		link.click()
		document.body.removeChild(link)
		setIsDownloading(false)
	}	

	const onPlay = async () => {
		try {
			if (ref.current && !didDownload) {
				const audioPlayer = ref.current.audioEl.current

				audioPlayer.pause()
				setIsLoading(true)
				const downloadUrl = await loadFile(props.src)
				setDidDownload(true)
				setIsLoading(false)

				audioPlayer.src = downloadUrl
				audioPlayer.load()
				audioPlayer.play()
			}
		} catch (err) {
			console.log('err', err)
		}
	}


	return (
		<SAudioPlayer>
			<ReactAudioPlayer {...props} controlsList={"nodownload"} onPlay={onPlay} src={props.src} ref={ref} />
			<AudioTitleWrapper>
				<span>{props.attachment.filename}</span>
				<span>{filesize(props.attachment['Content-Length'])}</span>
			</AudioTitleWrapper>
			<Button
				  style={{
					  width: '100%',
					  fontSize: 10,
					  padding: '2px 4px',
					  marginTop: '0px',
				  }}
				  disabled={isDownloading}
				  primary
				  onClick={() => downloadImage(props.src, props.attachment.filename)}
			  >Download Audio</Button>
		</SAudioPlayer>
	)
}

export default forwardRef(AudioPlayer)