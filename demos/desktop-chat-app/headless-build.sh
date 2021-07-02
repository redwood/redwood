#!/bin/bash

cd ../.. && \
docker build --rm -t brynbellomy/redwood-chat --file ./demos/desktop-chat-app/headless.Dockerfile . && \
docker push brynbellomy/redwood-chat && \
cd -
