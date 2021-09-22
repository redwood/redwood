import React, { useState, useRef, Fragment } from 'react'
import styled from 'styled-components'
import filesize from 'filesize.js'

import VideoJS from './../VideoJS'
import loadingGoo from './../Account/assets/loading-goo.svg'
import Button from './../Button'
import reloadIcon from './../../assets/reload.svg'

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

const SVideoPreviewWrapper = styled.div`
	position: relative;
`

const VideoTitleWrapper = styled.div`
	display: flex;
	background: black;
	margin-top: 8px;
	justify-content: space-between;
	padding-left: 8px;
	padding-right: 8px;
	padding-top: 4px;
	padding-bottom: 8px;
	span {
		font-size: 10px;
		color: rgba(255, 255, 255, .8);
		&:nth-child(2) {
			color: rgba(255, 255, 255, .4);
		}
	}
`

const VideoLoadingOverlay = styled.div`
	position: absolute;
	width: 100%;
	height: 100%;
	background: rgba(0, 0, 0, .6);
	top: 0px;
	display: flex;
	align-items: center;
	justify-content: center;
	flex-direction: column;
	> img {
		height: 108px;
	}
	> span {
		font-size: 14px;
		font-weight: 500;
	}
`

function VideoLoading() {
	return (
		<VideoLoadingOverlay>
			<img src={loadingGoo} alt="Loading" />
			<span>Loading video...</span>
		</VideoLoadingOverlay>
	)
}

function VideoDownloading() {
	return (
		<VideoLoadingOverlay>
			<img src={loadingGoo} alt="Loading" />
			<span>Downloading video...</span>
		</VideoLoadingOverlay>
	)
}

const SPlayOverlay = styled.div`
	position: absolute;	
	height: 100%;
	width: 100%;
	top: 0px;
	background: transparent;
	z-index: 999;
	cursor: pointer;
`

const SErrorOverlay = styled.div`
	position: absolute;	
	height: 100%;
	width: 100%;
	top: 0px;
	background: rgba(0,0,0, .9);
	z-index: 999;
	cursor: pointer;
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	cursor: pointer;
	&:hover {
		img {
			transform: rotate(360deg) scale(0.8);
		}
	}
	span {
		color: rgba(255, 255, 255, .8);
		&:nth-child(3) {
			font-weight: 500;
			font-size: 16px;
		}
	}
	> img {
		transition: all ease-in-out .25s;
		height: 72px;
		margin-top: 8px;
		margin-bottom: 8px;
	}
`


function VideoPreview(props) {
	const { url, attachment } = props
	const [didDownload, setDidDownload] = useState(false)
	const [isLoading, setIsLoading] = useState(false)
	const [isDownloading, setIsDownloading] = useState(false)
	const [hasError, setHasError] = useState(false)
	const [blobUrl, setBlobUrl] = useState(url)
	const videoRef = useRef(null)

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

	const onClick = async (retry) => {
		try {
			if (isLoading) { return }
			if (retry || (videoRef.current && !didDownload)) {
				setHasError(false)
				videoRef.current.player.pause()
				setIsLoading(true)
				const downloadUrl = await loadFile(url)
				setBlobUrl(downloadUrl)
				setDidDownload(true)
				setIsLoading(false)
				
				videoRef.current.player.src({ type: attachment['Content-Type'], src: downloadUrl })
				videoRef.current.player.load()
				videoRef.current.player.play()
			}
		} catch (err) {
			setHasError(true)
			console.log('err', err)
		}
	}

	const onError = () => {
		setHasError(true)
	}

	const videoJsOptions = {
		autoplay: false,
		playbackRates: [0.5, 1, 1.25, 1.5, 2],
		width: 500,
		height: 250,
		controls: true,
		sources: [
		  {
			src: url,
			type: attachment['Content-Type'],
		  },
		],
	  };

	  return (
		  <Fragment>
			  <SVideoPreviewWrapper>
				  <VideoTitleWrapper>
					  <span>{attachment.filename}</span>
					  <span>{filesize(attachment['Content-Length'])}</span>
				  </VideoTitleWrapper>
				  <VideoJS ref={videoRef} onError={onError} {...videoJsOptions} />
				  { hasError ? (
					  <SErrorOverlay onClick={() => onClick(true)}>
						  <span>Problem loading video</span>
						  <img alt="Reload" src={reloadIcon} />
						  <span>Click to retry</span>
					  </SErrorOverlay>
				  ) : null}
				  { !didDownload ? <SPlayOverlay onClick={() => onClick()} /> : null }
				  {isLoading ? <VideoLoading /> : null}
				  {isDownloading ? <VideoDownloading /> : null}
			  </SVideoPreviewWrapper>
			  <Button
				  style={{
					  width: '100%',
					  fontSize: 12,
					  padding: '4px 8px'
				  }}
				  disabled={isDownloading}
				  primary
				  onClick={() => downloadImage(url, attachment.filename)}
			  >Download Video</Button>
		  </Fragment>
	  )
}

export default VideoPreview