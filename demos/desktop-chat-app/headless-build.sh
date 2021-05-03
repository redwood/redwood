#!/bin/bash

cd ../.. && \
docker build -t brynbellomy/redwood-chat --file ./demos/desktop-chat-app/headless.Dockerfile . && \
docker push brynbellomy/redwood-chat && \
cd -