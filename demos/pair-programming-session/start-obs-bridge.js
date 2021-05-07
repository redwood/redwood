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

    let uploaded = {}

    const upload = _.debounce(async () => {
        let patches =
            (await Promise.all(
                fs.readdirSync(path.join(__dirname, 'recordings', 'live', 'stream')).map(file => {
                    return path.join(__dirname, 'recordings', 'live', 'stream', file)
                }).filter(fullpath => {
                    return !uploaded[fullpath] || fs.statSync(fullpath).size !== uploaded[fullpath].size
                }).map(fullpath => {
                    let file = fs.createReadStream(fullpath)
                    return Promise.all([ fullpath, braidClient.storeRef(file).then(hashes => hashes.sha3) ])
                })
            )).map(([ fullpath, sha3 ]) => {
                uploaded[fullpath] = { size: fs.statSync(fullpath).size, sha3 }
                console.log(fullpath, sha3, uploaded[fullpath].size)
                return `.streams.${braidClient.identity.address}["${path.basename(fullpath)}"] = ` + Braid.utils.JSON.stringify({
                    'Content-Type': 'link',
                    'value': `blob:sha3:${sha3}`,
                })
            })

        try {
            let txID = Braid.utils.randomID()
            await braidClient.put({
                stateURI: 'p2pair.local/video',
                id: txID,
                parents: [ parentTxID ],
                patches: patches,
            })
            parentTxID = txID

        } catch (err) {
            console.log('error ~>', err)
        }
    }, 500)

    // Set up the media server that captures output from OBS Studio
    const mediaServer = new NodeMediaServer({
        rtmp: {
            port: 1935,
            chunk_size: 20000,
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
                    hlsFlags: '[hls_time=2:hls_list_size=0]',
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

    watch('./recordings/live/stream', {}, upload)
    mediaServer.run()
})()