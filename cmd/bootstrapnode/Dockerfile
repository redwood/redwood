FROM node:latest AS embed

ADD ./embed /build/embed/
WORKDIR /build/embed
RUN yarn




FROM golang:1.18.0-buster AS build

RUN apt update
RUN apt -y install build-essential
RUN apt -y install pkg-config

ADD go.mod go.sum /build/
ADD ./blob /build/blob/
ADD ./cmd /build/cmd/
ADD ./crypto /build/crypto/
ADD ./embed /build/embed/
ADD ./errors /build/errors/
ADD ./health /build/health/
ADD ./identity /build/identity/
ADD ./internal /build/internal/
ADD ./log /build/log/
ADD ./process /build/process/
ADD ./rpc /build/rpc/
ADD ./state /build/state/
ADD ./swarm /build/swarm/
ADD ./tree /build/tree/
ADD ./types /build/types/
ADD ./utils /build/utils/

COPY --from=embed /build/embed/node_modules /build/embed/node_modules

WORKDIR /build/cmd/bootstrapnode
RUN go get -d
RUN go build -o /bootstrapnode .




FROM golang:1.18.0-buster

COPY --from=build /bootstrapnode /bootstrapnode
WORKDIR /
CMD ["/bootstrapnode"]