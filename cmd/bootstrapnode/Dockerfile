FROM golang:1.18.0-buster AS build

RUN apt update
RUN apt -y install build-essential
RUN apt -y install pkg-config

ADD . /build/

WORKDIR /build/cmd/bootstrapnode
RUN go get -d
RUN go build -o /bootstrapnode .




FROM golang:1.18.0-buster

COPY --from=build /bootstrapnode /bootstrapnode
WORKDIR /
CMD ["/bootstrapnode"]