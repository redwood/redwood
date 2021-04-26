FROM golang:1.15.7-buster AS build

RUN apt update
RUN apt -y install build-essential
RUN apt -y install pkg-config

ADD . /build/

WORKDIR /build/demos/desktop-chat-app
RUN go get -d
RUN go build --tags headless -o /redwood-chat .




FROM golang:1.15.7-buster

COPY --from=build /redwood-chat /redwood-chat
WORKDIR /
CMD ["/redwood-chat"]
