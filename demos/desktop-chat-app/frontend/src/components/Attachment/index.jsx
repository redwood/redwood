import React, { useState, useCallback, useRef, useEffect } from 'react'
import styled, { useTheme } from 'styled-components'
import filesize from 'filesize.js'
import Embed from '../Embed'
import { isImage, isPDF } from '../../utils/contentTypes'

const ImageWrapper = styled.div`
    padding: 4px 0;
    cursor: pointer;
`

const EmbedWrapper = styled.div`
    padding: 4px 4px;
    background-color: ${props => props.theme.color.grey[500]};
    border-radius: 8px;
    cursor: pointer;
`

const Metadata = styled.div`
    padding-bottom: 4px;
`

const Filename = styled.span`
    font-size: 0.7rem;
`

const Filesize = styled.span`
    font-size: 0.8rem;
    color: ${props => props.theme.color.grey[100]};
`

const SDownloadLink = styled.a`
    color: ${props => props.theme.color.white};
    text-decoration: none;

    &:hover {
        color: blue;
        text-decoration: underline;
    }
`

function Attachment({ attachment, url, onClick, className }) {
    const _onClick = useCallback(() => {
        onClick(attachment, url)
    }, [attachment, url, onClick])

    let Wrapper
    if (isImage(attachment['Content-Type'])) {
        Wrapper = ImageWrapper
    } else {
        Wrapper = EmbedWrapper
    }

    return (
        <Wrapper className={className} onClick={_onClick}>
            <Metadata>
                <Filename><SDownloadLink href={url} target="_blank">{attachment.filename}</SDownloadLink> </Filename>
                <Filesize>({filesize(attachment['Content-Length'])})</Filesize>
            </Metadata>
            <Embed contentType={attachment['Content-Type']} url={url} width={200} />
        </Wrapper>
    )
}

function DownloadLink({ href, target, children }) {
    function onClick(e) {
        e.stopPropagation()
    }
    return (
        <a href={href} target={target} onClick={onClick}>{children}</a>
    )
}

export default Attachment
