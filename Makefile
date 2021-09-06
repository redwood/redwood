

.PHONY: redwood
redwood: redwood.js/dist
	go build --tags static ./cmd/redwood

redwood.js/dist:
	cd redwood.js && \
	yarn && \
	yarn build:main


.PHONY: redwood-chat
redwood-chat: demos/desktop-chat-app/frontend/build
	cd ./demos/desktop-chat-app && \
	go build -o $(GOPATH)/bin/redwood-chat .

demos/desktop-chat-app/frontend/node_modules:
	cd demos/desktop-chat-app/frontend && \
	yarn install

demos/desktop-chat-app/frontend/build: demos/desktop-chat-app/frontend/node_modules
	cd demos/desktop-chat-app/frontend && \
	yarn build
