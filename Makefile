

.PHONY: build
build: redwood.js/dist
	go build --tags static ./cmd/redwood

redwood.js/dist:
	cd redwood.js && \
	yarn && \
	yarn build:main

