

.PHONY: redwood
redwood: redwood.js/dist
	go build ./cmd/redwood

redwood.js/dist:
	cd redwood.js && \
	yarn && \
	yarn build:main


.PHONY: hush
hush: demos/desktop-chat-app/frontend/build
	cd ./demos/desktop-chat-app && \
	go build -o $(GOPATH)/bin/hush .

demos/desktop-chat-app/frontend/node_modules:
	cd demos/desktop-chat-app/frontend && \
	yarn install

demos/desktop-chat-app/frontend/build: demos/desktop-chat-app/frontend/node_modules
	cd demos/desktop-chat-app/frontend && \
	yarn build

.PHONY: redwood-docker
redwood-docker:
	docker build -t redwoodp2p/redwood --file ./Dockerfile .

.PHONY: bootstrapnode-docker
bootstrapnode-docker:
	docker build -t redwoodp2p/bootstrapnode --file ./cmd/bootstrapnode/Dockerfile .
