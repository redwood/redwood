#!/bin/sh

apk add docker
rc-update add docker boot
service docker start
mkdir -p /root/config

echo <<<EOF > /root/config/.redwoodrc
Node:
    SubscribedStateURIs:
      - ${SUBSCRIBED_STATE_URI}
    DevMode: true
P2PTransport:
    Enabled: true
    ListenAddr: 0.0.0.0
    ListenPort: 21231
HTTPTransport:
    Enabled: true
    ListenHost: :8080
HTTPRPC:
    Enabled: true
    ListenHost: :8081
    Whitelist:
        Enabled: true
        PermittedAddrs:
          - ${ADMIN_ADDRESS}
EOF

docker run -v /root/config:/config -p 8080:8080 -p 8081:8081 -p 21231:21231 brynbellomy/redwood-chat
