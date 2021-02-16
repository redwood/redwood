FROM golang:1.15.7-buster AS build

RUN apt update
RUN apt -y install build-essential
RUN apt -y install pkg-config

ADD go.mod go.sum /app/
ADD *.go /app/
ADD nelson/*.go /app/nelson/
ADD ctx/*.go /app/ctx/
ADD types/*.go /app/types/
ADD tree/*.go /app/tree/
WORKDIR /app
RUN go get -d

ADD . /app
WORKDIR /app/demos/desktop-chat-app
RUN go build --tags headless -o /redwood-chat .




FROM golang:1.15.7-buster

COPY --from=build /redwood-chat /redwood-chat
WORKDIR /
CMD ["/redwood-chat", "--dev", "--config", "/config/.redwoodrc"]
