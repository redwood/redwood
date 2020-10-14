import React, { useState, useEffect, useRef } from 'react'
import { braidClient } from './index'
import videojs from 'video.js'
import 'video.js/dist/video-js.css';
let Braid = window.Braid

function VideoPlayer() {
    let [state, setState] = useState({})

    useEffect(() => {
        braidClient.subscribe({
            stateURI: 'p2pair.local/video',
            keypath:  '/',
            txs:      true,
            states:   true,
            fromTxID: Braid.utils.genesisTxID,
            callback: (err, { tx, state: newTree, leaves } = {}) => {
                console.log('video ~>', err, {tx, state, leaves})
                if (err) return console.error(err)
                setState({ ...state, ...newTree })
            },
        })
    }, [])

    return (
        <div>
            {state.streams && Object.keys(state.streams).map(userAddr => (
                <Video source={`/streams/${userAddr}/index.m3u8?state_uri=p2pair.local/video`} key={userAddr} />
            ))}
            <script src="https://unpkg.com/browse/@videojs/http-streaming@1.13.3/dist/videojs-http-streaming.min.js"></script>
        </div>
    )
}

function Video({ source }) {
    const playerRef = useRef(null)

    useEffect(() => {
        const player = videojs(playerRef.current, { controls: true, muted: true }, () => {
            player.src(source)
        })
        return () => { player.dispose() }
    }, [source])

    return (
        <div data-vjs-player>
            <video ref={playerRef} className="video-js vjs-16-9" playsInline />
        </div>
    )
}

export default VideoPlayer
