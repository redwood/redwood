FROM golang:1.17.1-buster AS build

RUN apt update
RUN apt -y install build-essential
RUN apt -y install pkg-config

ADD go.mod go.sum /build/
ADD ./blob /build/blob/
ADD ./cmd /build/cmd/
ADD ./crypto /build/crypto/
ADD ./errors /build/errors/
ADD ./health /build/health/
ADD ./identity /build/identity/
ADD ./internal /build/internal/
ADD ./log /build/log/
ADD ./process /build/process/
ADD ./redwood.js /build/redwood.js/
ADD ./rpc /build/rpc/
ADD ./state /build/state/
ADD ./swarm /build/swarm/
ADD ./tree /build/tree/
ADD ./types /build/types/
ADD ./utils /build/utils/

WORKDIR /build/cmd/redwood
RUN go get -d

RUN go build -o /redwood .




FROM golang:1.17.1-buster

COPY --from=build /redwood /redwood
WORKDIR /
CMD ["/redwood", "--config", "/config/.redwoodrc", "--password-file", "/config/password.txt"]
