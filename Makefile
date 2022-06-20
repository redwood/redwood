
.PHONY: redwood
redwood:
	cd embed && npm install && cd .. && \
	go build -o $(GOPATH)/bin/redwood ./cmd/redwood

.PHONY: hush
hush:
	cd embed && npm install && cd .. && \
	cd ./demos/desktop-chat-app && \
	cd frontend && NODE_OPTIONS=--openssl-legacy-provider yarn install && NODE_OPTIONS=--openssl-legacy-provider yarn build && cd .. && \
	rm -rf ./output && \
	astilectron-bundler && \
	cd ./output/linux-amd64 && zip -r ../hush-linux.zip ./Hush && \
	cd ../windows-amd64 && zip -r ../hush-windows.zip ./Hush.exe && \
	cd ../darwin-amd64 && zip -r ../hush-macos.zip ./Hush.app

.PHONY: bootstrapnode
bootstrapnode:
	go build ./cmd/bootstrapnode

.PHONY: hush-upload
hush-upload:
	gh release upload $(TAG) ./demos/desktop-chat-app/output/hush-linux.zip ./demos/desktop-chat-app/output/hush-windows.zip ./demos/desktop-chat-app/output/hush-macos.zip --clobber

.PHONY: redwood-docker
redwood-docker:
	docker build -t redwoodp2p/redwood --file ./Dockerfile .

.PHONY: bootstrapnode-docker
bootstrapnode-docker:
	docker build -t redwoodp2p/bootstrapnode --file ./cmd/bootstrapnode/Dockerfile .

