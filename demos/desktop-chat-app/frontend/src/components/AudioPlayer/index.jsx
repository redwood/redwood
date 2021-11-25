import React, { forwardRef, useState, Fragment } from 'react'
import styled from 'styled-components'
import filesize from 'filesize.js'
import AudioM5Player from 'react-h5-audio-player'
import 'react-h5-audio-player/lib/styles.css'

import Button from '../Button'
import loadingGoo from '../Account/assets/loading-goo.svg'
import reloadIcon from '../../assets/reload.svg'

import './audio-player.css'

const loadFile = async (url) => {
    try {
        const resp = await fetch(url, {
            method: 'GET',
        })
        const blob = await resp.blob()
        const downloadUrl = window.URL.createObjectURL(blob)
        return downloadUrl
    } catch (err) {
        return err
    }
}

const AudioLoadingOverlay = styled.div`
    position: absolute;
    width: 100%;
    height: 100%;
    background: rgba(0, 0, 0, 0.6);
    top: 0px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-direction: column;
    border-top-left-radius: 4px;
    border-top-right-radius: 4px;
    > img {
        height: 60px;
        transform: scale(1.5);
    }
    > span {
        font-size: 12px;
        font-weight: 500;
    }
`

function AudioLoading() {
    return (
        <AudioLoadingOverlay>
            <img src={loadingGoo} alt="Loading" />
            <span>Loading audio...</span>
        </AudioLoadingOverlay>
    )
}

function AudioDownloading() {
    return (
        <AudioLoadingOverlay>
            <img src={loadingGoo} alt="Loading" />
            <span>Downloading audio...</span>
        </AudioLoadingOverlay>
    )
}

const SAudioPlayer = styled.div`
    margin-top: 12px;
    display: flex;
    flex-direction: column;
    position: relative;
`

const AudioTitleWrapper = styled.div`
    display: flex;
    background: #1c1e21;
    justify-content: space-between;
    padding-left: 8px;
    padding-right: 8px;
    padding-top: 4px;
    padding-bottom: 8px;
    span {
        font-size: 10px;
        color: rgba(255, 255, 255, 0.8);
        &:first-child {
            max-width: 400px;
            text-overflow: ellipsis;
            overflow: hidden;
            text-align: center;
            white-space: nowrap;
        }
        &:nth-child(2) {
            color: rgba(255, 255, 255, 0.4);
        }
    }
`

const SErrorOverlay = styled.div`
    position: absolute;
    height: 100%;
    width: 100%;
    top: 0px;
    background: rgba(0, 0, 0, 0.9);
    z-index: 999;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    border-top-left-radius: 4px;
    border-top-right-radius: 4px;
    &:hover {
        img {
            transform: rotate(360deg) scale(0.8);
        }
    }
    span {
        color: rgba(255, 255, 255, 0.8);
        font-size: 10px;
        &:nth-child(3) {
            font-weight: 500;
            font-size: 12px;
        }
    }
    > img {
        transition: all ease-in-out 0.25s;
        height: 36px;
        margin-top: 8px;
        margin-bottom: 8px;
    }
`

function AudioPlayer(props, ref) {
    const { attachment, src } = props
    const [didDownload, setDidDownload] = useState(false)
    const [isLoading, setIsLoading] = useState(false)
    const [hasError, setHasError] = useState(false)
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

    const onPlay = async (retry) => {
        try {
            if (retry || (ref.current && !didDownload)) {
                setHasError(false)
                const audioPlayer = ref.current.audio.current

                audioPlayer.pause()
                setIsLoading(true)
                const downloadUrl = await loadFile(retry ? src : 'url')
                setDidDownload(true)
                setIsLoading(false)

                audioPlayer.src = downloadUrl
                audioPlayer.load()
                audioPlayer.play()
            }
        } catch (err) {
            setHasError(true)
            return err
        }
    }

    return (
        <>
            <SAudioPlayer>
                <AudioM5Player
                    src={src}
                    onPlay={() => onPlay()}
                    ref={ref}
                    onError={() => setHasError(true)}
                />
                <AudioTitleWrapper>
                    <span>{attachment.filename}</span>
                    <span>{filesize(props.attachment['Content-Length'])}</span>
                </AudioTitleWrapper>
                {hasError ? (
                    <SErrorOverlay onClick={() => onPlay(true)}>
                        <span>Problem loading audio</span>
                        <img alt="Reload" src={reloadIcon} />
                        <span>Click to retry</span>
                    </SErrorOverlay>
                ) : null}
                {isLoading ? <AudioLoading /> : null}
                {isDownloading ? <AudioDownloading /> : null}
            </SAudioPlayer>
            <Button
                style={{
                    width: '100%',
                    fontSize: 10,
                    padding: '2px 4px',
                    marginTop: '0px',
                    borderTopLeftRadius: '0px',
                    borderTopRightRadius: '0px',
                }}
                disabled={isDownloading}
                primary
                onClick={() => downloadImage(src, attachment.filename)}
            >
                Download Audio
            </Button>
        </>
    )
}

export default forwardRef(AudioPlayer)
