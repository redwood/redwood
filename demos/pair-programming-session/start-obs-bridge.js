const fs = require('fs')
const path = require('path')
const watch = require('node-watch')
const NodeMediaServer = require('node-media-server')
const _ = require('lodash')
const yaml = require('js-yaml')
const Braid = require('../../braidjs/braid-src.js')

;(async function() {
    //
    // Braid setup
    //
    const mnemonic = yaml.safeLoad(fs.readFileSync('./node1.redwoodrc', 'utf8')).Node.HDMnemonicPhrase
    let braidClient = Braid.createPeer({
        identity: Braid.identity.fromMnemonic(mnemonic),
        httpHost: 'http://localhost:8080',
        onFoundPeersCallback: (peers) => {},
    })

    //
    // Media server setup
    //
    await braidClient.authorize()

    // Set up our file watcher / uploader
    let parentTxID = Braid.utils.genesisTxID

    const upload = _.debounce(async (evt, filename) => {
        let file = fs.createReadStream(filename)
        let { sha3: fileSHA } = await braidClient.storeRef(file)

        try {
            let indexM3U8 = fs.createReadStream(path.join(__dirname, 'recordings', 'live', 'asdf', 'index.m3u8'))
            let { sha3: indexM3U8SHA } = await braidClient.storeRef(indexM3U8)

            let txID = Braid.utils.randomID()
            console.log('trying to send tx')
            await braidClient.put({
                stateURI: 'p2pair.local/video',
                id: txID,
                parents: [ parentTxID ],
                patches: [
                    `.streams.${braidClient.identity.address}["index.m3u8"] = ` + Braid.utils.JSON.stringify({
                        'Content-Type': 'link',
                        'value': `ref:sha3:${indexM3U8SHA}`,
                    }),
                    `.streams.${braidClient.identity.address}["${path.basename(filename)}"] = ` + Braid.utils.JSON.stringify({
                        'Content-Type': 'link',
                        'value': `ref:sha3:${fileSHA}`,
                    }),
                ],
            })
            parentTxID = txID

        } catch (err) {
            console.error(err)

            let txID = Braid.utils.randomID()
            await braidClient.put({
                stateURI: 'p2pair.local/video',
                id: txID,
                parents: [ parentTxID ],
                patches: [
                    `.streams.${braidClient.identity.address}["${path.basename(filename)}"] = ` + Braid.utils.JSON.stringify({
                        'Content-Type': 'link',
                        'value': `ref:sha3:${fileSHA}`,
                    }),
                ],
            })
            parentTxID = txID
        }
    }, 500)

    // Set up the media server that captures output from OBS Studio
    const mediaServer = new NodeMediaServer({
        rtmp: {
            port: 1935,
            chunk_size: 60000,
            gop_cache: true,
            ping: 60,
            ping_timeout: 30,
        },
        http: {
            port: 8888,
            mediaroot: './recordings',
            allow_origin: '*',
        },
        trans: {
            ffmpeg: '/usr/local/bin/ffmpeg',
            tasks: [
                {
                    app: 'live',
                    hls: true,
                    hlsFlags: '[hls_time=2:hls_list_size=3:hls_flags=delete_segments]',
                    dash: true,
                    dashFlags: '[f=dash:window_size=3:extra_window_size=5]',
                    mp4: true,
                    mp4Flags: '[movflags=faststart]',
                },
            ],
        },
    })

    mediaServer.on('prePublish', async (id, StreamPath, args) => {
        console.log('prepublish ~>', { id, StreamPath, args })
        let stream_key = getStreamKeyFromStreamPath(StreamPath)
        console.log(stream_key)
        console.log('[NodeEvent on prePublish]', `id=${id} StreamPath=${StreamPath} args=${JSON.stringify(args)}`)
    })

    function getStreamKeyFromStreamPath(path) {
        let parts = path.split('/')
        return parts[parts.length - 1]
    }

    watch('./recordings/live/asdf', {}, upload)
    mediaServer.run()
})()