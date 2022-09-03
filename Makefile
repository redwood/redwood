
.PHONY: redwood
redwood:
	cd embed && npm install && cd .. && \
	go build -o $(GOPATH)/bin/redwood ./cmd/redwood

.PHONY: relay
relay:
	go build ./cmd/relay

.PHONY: vault
vault:
	cd embed && npm install && cd .. && \
	go build -o $(GOPATH)/bin/vault ./cmd/vault

.PHONY: hush
hush:
	cd embed && npm install && cd .. && \
	cd ./demos/desktop-chat-app && \
	cd frontend &&  yarn install &&  yarn build && cd .. && \
	rm -rf ./output && \
	astilectron-bundler && \
	cd ./output/linux-amd64 && zip -r ../hush-linux.zip ./Hush && \
	cd ../windows-amd64 && zip -r ../hush-windows.zip ./Hush.exe
# 	&& \
# 	cd ../darwin-amd64 && zip -r ../hush-macos.zip ./Hush.app

.PHONY: hush-upload
hush-upload:
	gh release upload $(TAG) ./demos/desktop-chat-app/output/hush-linux.zip ./demos/desktop-chat-app/output/hush-windows.zip --clobber

.PHONY: redwood-docker
redwood-docker:
	docker build -t redwoodp2p/redwood --file ./Dockerfile .

.PHONY: relay-docker
relay-docker:
	docker build -t redwoodp2p/relay --file ./cmd/relay/Dockerfile .

.PHONY: vault-docker
vault-docker:
	docker build -t redwoodp2p/vault --file ./cmd/vault/Dockerfile .

