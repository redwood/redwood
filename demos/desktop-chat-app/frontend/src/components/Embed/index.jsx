import React, { useState, useCallback, useRef, useEffect } from 'react'
import styled, { useTheme } from 'styled-components'
import filesize from 'filesize.js'
import { usePdf } from '@mikecousins/react-pdf'
import { IconButton, Avatar } from '@material-ui/core'
import Image from '../Image'
import { isImage, isPDF } from '../../utils/contentTypes'

const Wrapper = styled.div`
	width: ${props => props.width}px;
	cursor: pointer;
`

const SImage = styled(Image)`
	height: ${props => props.height || 'auto'};
    width: ${props => props.height ? 'auto' : '100%'};
`

function Embed({ contentType, url, height, width, className, loadFailed, setLoadFailed, onClick }) {
    let content
    if (isImage(contentType)) {
		return <SImage
			loadFailed={loadFailed}
			setLoadFailed={setLoadFailed}
			onClick={onClick}
			src={url}
			height={height}
			className={className}
		/>
    } else if (isPDF(contentType)) {
        return <PDF url={url} width={width} className={className} />
    }
    return null
}

function PDF({ url, width }) {
    const canvasRef = useRef(null)
    const [scale, setScale] = useState(1)
    const { pdfDocument, pdfPage } = usePdf({ file: url, page: 1, canvasRef, scale })

    useEffect(() => {
        if (!pdfPage) {
            return
        }
        const x = pdfPage.getViewport()
        const { viewBox } = x
        if (!viewBox) {
            return
        }
        const actualWidth = viewBox[2]
        setScale(width / actualWidth)
    }, [url, pdfPage, setScale])

    let height = 0
    if (!!pdfPage) {
        const x = pdfPage.getViewport()
        const { viewBox } = x
        if (!viewBox) {
            return
        }
        height = viewBox[3] * scale
    }

    return (
        <canvas style={{ width: width, maxWidth: width, height, maxHeight: height }} ref={canvasRef} />
    )
}

export default Embed
