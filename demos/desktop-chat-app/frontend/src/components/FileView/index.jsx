import React, { useState, useCallback } from 'react'
import styled, { useTheme } from 'styled-components'
import { useRedwood, useStateTree } from '@redwood.dev/react'
import { CloudDownload as DownloadIcon, OpenInNew as OpenIcon } from '@material-ui/icons'
import AttachmentModal from '../AttachmentModal'
import Embed from '../Embed'
import Scrollbars from '../Scrollbars'
import useModal from '../../hooks/useModal'
import useNavigation from '../../hooks/useNavigation'

let imageWidth = 220
let imageHeight = 175

const SFileView = styled.div`
    background-color: ${props => props.backgroundColor};
    border-left: 2px solid ${props => props.borderColor};
    font-weight: 300;
    // height: calc(100vh - 90px);
    height: 100%;
    display: flex;
    flex-direction: column;
    // box-shadow: inset 1px 3px 14px 0px rgb(0 0 0 / 19%);
`

const SHeader = styled.h3`
    margin: 0;
    padding: 16px;
    // box-shadow: 0px 10px 11px -2px rgba(0,0,0,0.39);
`

const SFilesContainer = styled.div`
    display: flex;
    flex-grow: 1;
`

const SFilesContainerInner = styled.div`
    padding: 20px;
    display: flex;
    flex-direction: row;
    flex-wrap: wrap;
    justify-content: flex-start;
    align-items: flex-start;
    align-content: flex-start;
    flex-grow: 1;
`

const SIconLink = styled.a`
    color: white;
`

const SFileContainer = styled.div`
    flex-grow: 0;
    flex-shrink: 0;
    width: ${_ => imageWidth}px;
    height: calc(${ _ => imageHeight}px + 25px);
    margin: 8px;
    padding:  8px;
    border-radius: 6px;
    background-color: ${props => props.backgroundColor};
`

const SFilename = styled.div`
    height: 24px;
    margin-top: -4px;
    margin-bottom: 4px;
`

const SEmbedWrapper = styled.div`
    position: relative;
    width: ${_ => imageWidth}px;
    height: ${_ => imageHeight}px;
    border-radius: 6px;
    overflow: hidden;
`

const SFileOverlay = styled.div`
    display: flex;
    align-items: center;
    justify-content: center;
    width: ${_ => imageWidth}px;
    height: ${_ => imageHeight}px;
    position: absolute;
    top: 0;
    left: 0;
    background-color: rgba(0, 0, 0, 0.6);
    border-radius: 6px;
    opacity: 0;
    overflow: hidden;

    &:hover {
        opacity: 1;
    }
`

const SEmbed = styled(Embed)`
    width: ${_ => imageWidth}px;
    height: ${_ => imageHeight}px;
`

const SDownloadIcon = styled(DownloadIcon)`
    width: 48px !important;
    height: 48px !important;
    cursor: pointer;
    margin-right: 16px;
`

const SOpenIcon = styled(OpenIcon)`
    width: 40px !important;
    height: 40px !important;
    cursor: pointer;
`

function FileView({ className }) {
    const theme = useTheme()
    const { selectedStateURI } = useNavigation()
    const [roomState] = useStateTree(selectedStateURI)
    const { httpHost } = useRedwood()
    const { onPresent: onPresentPreviewModal } = useModal('attachment preview')

    const onClickAttachment = useCallback((attachment, url) => {
        onPresentPreviewModal({ attachment, url })
    }, [onPresentPreviewModal])

    let files = (roomState || {}).files || []

    return (
        <SFileView borderColor={theme.color.grey[300]} backgroundColor={theme.color.grey[200]} className={className}>
            <SFilesContainer borderColor={theme.color.grey[300]}>
                <Scrollbars shadow style={{ flexGrow: 1 }}>
                    <SFilesContainerInner>
                    {files.map((file, i) => {
                        let url = `${httpHost}/files[${i}]?state_uri=${encodeURIComponent(selectedStateURI)}`
                        console.log('file', file)
                        return (
                            <SFileContainer backgroundColor={theme.color.grey[400]} key={selectedStateURI + file.filename + i}>
                                <SFilename>{file.filename}</SFilename>
                                <SEmbedWrapper>
                                    <SEmbed contentType={file['Content-Type']} url={url} />
                                    <SFileOverlay>
                                        {/*<SIconLink download={file.filename} href={url}><SDownloadIcon /></SIconLink>*/}
                                        <SIconLink onClick={() => onClickAttachment(file, url)}><SOpenIcon /></SIconLink>
                                    </SFileOverlay>
                                </SEmbedWrapper>
                            </SFileContainer>
                        )
                    })}
                    </SFilesContainerInner>
                </Scrollbars>
            </SFilesContainer>
        </SFileView>
    )
}

export default FileView
