import React, { useState, useRef, useEffect, memo } from 'react'
import styled from 'styled-components'
import { usePdf } from '@mikecousins/react-pdf'
import Image from '../Image'
import { isImage, isPDF } from '../../utils/contentTypes'

const SImage = memo(styled(Image)`
    height: ${(props) => props.height || 'auto'};
    width: ${(props) => (props.height ? 'auto' : '100%')};
`)

function Embed({
    contentType,
    url,
    height,
    width,
    className,
    loadFailed,
    setLoadFailed,
    onClick,
}) {
    if (isImage(contentType)) {
        return (
            <SImage
                loadFailed={loadFailed}
                setLoadFailed={setLoadFailed}
                onClick={onClick}
                src={url}
                height={height}
                className={className}
            />
        )
    }
    if (isPDF(contentType)) {
        return <PDF url={url} width={width} className={className} />
    }
    return null
}

function PDF({ url, width }) {
    const canvasRef = useRef(null)
    const [scale, setScale] = useState(1)
    const { pdfPage } = usePdf({
        file: url,
        page: 1,
        canvasRef,
        scale,
    })

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
    }, [url, pdfPage, setScale, width])

    let height = 0
    if (pdfPage) {
        const x = pdfPage.getViewport()
        const { viewBox } = x
        if (!viewBox) {
            return null
        }
        height = viewBox[3] * scale
    }

    return (
        <canvas
            style={{ width, maxWidth: width, height, maxHeight: height }}
            ref={canvasRef}
        />
    )
}

export default memo(Embed)
