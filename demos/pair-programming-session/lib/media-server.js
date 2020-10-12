const NodeMediaServer = require('node-media-server'),
    config = require('./config-rtmp-server.js');

const nms = new NodeMediaServer(config);

nms.on('prePublish', async (id, StreamPath, args) => {
    console.log('prepublish ~>', { id, StreamPath, args })
    let stream_key = getStreamKeyFromStreamPath(StreamPath);
    console.log(stream_key)
    console.log('[NodeEvent on prePublish]', `id=${id} StreamPath=${StreamPath} args=${JSON.stringify(args)}`);
});

const getStreamKeyFromStreamPath = (path) => {
    let parts = path.split('/');
    return parts[parts.length - 1];
};

module.exports = nms;