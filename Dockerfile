FROM golang:1.20-alpine as build
WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY cmd/ts-restic-proxy/*.go ./cmd/ts-restic-proxy/

RUN mkdir ./out
RUN go build -o ./out ./cmd/ts-restic-proxy

FROM alpine:latest

ENV RESTIC_SERVER_URL="http://127.0.0.1:9234"

RUN mkdir /data

WORKDIR /

COPY --from=build /app/out /

EXPOSE 8000/tcp

ENTRYPOINT /ts-restic-proxy -restic-rest-server="$RESTIC_SERVER_URL" -data-dir="/data" -htpasswd-file="/.htpasswd"
