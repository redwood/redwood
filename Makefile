

.PHONY: build
build:
	go build --tags static ./cmd/redwood

.PHONY: build-js
build-js:
	cd redwood.js && \
	yarn && \
	yarn build:main

