const request = require('request')
// const { createProxyMiddleware } = require('http-proxy-middleware')
module.exports = function(app) {
    app.all(/.*/, (req, res, next) => {
        try {
            function isDevServerRoute() {
                if (req.url.startsWith('/static')) {
                    return true
                } else if (req.url.startsWith('/manifest.json')) {
                    return true
                } else if (req.method === 'GET' && req.url === '/' && !req.headers.subscribe) {
                    return true
                }
                return false
            }

            if (!isDevServerRoute()) {
                console.log(req.method, req.url, '(redwood)', req.headers)
                console.log('')
                request({
                    url: 'http://localhost:8080' + req.url,
                    method: req.method,
                    headers: req.headers,
                }).pipe(res)
            } else {
                console.log(req.method, req.url, '(dev server)')
                next()
            }

        } catch (err) {
            console.log('ERR', err)
        }
    })

  // app.use(
  //   '/chat',
  //   createProxyMiddleware({
  //     target: 'http://localhost:8080',
  //     changeOrigin: true,
  //     preserveHeaderKeyCase: true,
  //     onError: (err, req, res) => {
  //       res.writeHead(500, {
  //         'Content-Type': 'text/plain',
  //       })
  //       res.end('error: ' + err)
  //     },
  //   })
  // )
  // app.use(
  //   '/editor',
  //   createProxyMiddleware({
  //     target: 'http://localhost:8080',
  //     changeOrigin: true,
  //     preserveHeaderKeyCase: true,
  //     onError: (err, req, res) => {
  //       res.writeHead(500, {
  //         'Content-Type': 'text/plain',
  //       })
  //       res.end('error: ' + err)
  //     },
  //   })
  // )
}