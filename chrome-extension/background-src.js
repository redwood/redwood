
var Braid = require('../braidjs/braid-src')

chrome.webRequest.onResponseStarted.addListener(details => {
    if (!details.responseHeaders.find(x => x.name.toLowerCase() === 'subscribe')) {
        return
    }

    var url = details.url
    if (url[url.length-1] === '/') {
        url = url.slice(0, -1)
    }

    var theTab
    chrome.tabs.query({active: true, currentWindow: true}, function(tabs) {
        theTab = tabs[0]
    })

    //
    // Braid/sync9 setup
    //
    var identity = Braid.identity.random()

    var s9 = Braid.sync9.create()

    var queue = Braid.utils.createTxQueue(
        (from, vid, parents, patches) => Braid.sync9.resolve_state(s9, from, vid, parents, patches),
        async (tx, newState) => {
            chrome.tabs.sendMessage(theTab.id, {
                action: 'xyzzy',
                data: newState,
            }, function(err) {
                if (err) {
                    console.error('oh no ~>', err)
                }
            });
        }
    )

    var braidClient = Braid.createPeer({
        identity: identity,
        httpHost: 'https://localhost:21232',
        //webrtc: true,
        onFoundPeersCallback: (peers) => {
            console.log('found peers ~>', peers)
        }
    })

    //braidClient.authorize().then(() => {
        //console.log('braid authorized')
        braidClient.subscribe('localhost:21231', '/', [ Braid.utils.genesisTxID ], queue.defaultTxHandler)
    //})

}, {urls:['<all_urls>']}, ["extraHeaders", 'responseHeaders'])

