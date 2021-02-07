FROM golang:1.15.7-buster

RUN apt update
RUN apt -y install build-essential
RUN apt -y install pkg-config

ADD . /app
WORKDIR /app/demos/desktop-chat-app
RUN go build --tags headless -o /redwood-chat .

WORKDIR /
CMD ["./redwood-chat", "--dev"]
