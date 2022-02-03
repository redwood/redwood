

.PHONY: redwood
redwood: redwood.js/dist
	go build ./cmd/redwood

redwood.js/dist:
	cd redwood.js && \
	yarn && \
	yarn build:main


.PHONY: hush
hush:
	cd ./demos/desktop-chat-app && \
	rm -rf ./output && \
	astilectron-bundler && \
	cd ./output/linux-amd64 && zip -r ../hush-linux.zip ./Hush && \
	cd ../windows-amd64 && zip -r ../hush-windows.zip ./Hush.exe && \
	cd ../darwin-amd64 && zip -r ../hush-macos.zip ./Hush.app

.PHONY: hush-upload
hush-upload:
	gh release upload $(TAG) ./demos/desktop-chat-app/output/hush-linux.zip ./demos/desktop-chat-app/output/hush-windows.zip ./demos/desktop-chat-app/output/hush-macos.zip --clobber

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
