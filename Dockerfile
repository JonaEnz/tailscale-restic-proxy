FROM golang:1.21-alpine as build
WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY cmd/ts-restic-proxy/*.go ./cmd/ts-restic-proxy/

RUN mkdir ./out
RUN go build -o ./out ./cmd/ts-restic-proxy

FROM alpine:latest


RUN mkdir /data

WORKDIR /

COPY --from=build /app/out /

EXPOSE 8000/tcp

CMD [ "sh", "-c", "./ts-restic-proxy", "-restic-rest-server ${RESTIC_SERVER_URL}", "-data-dir /data", "-htpasswd-file /.htpasswd", "-ts-auth-key ${TAILSCALE_AUTH_KEY}", "-ts-login-server ${TAILSCALE_CONTROL_SERVER}", "-hostname ${TAILSCALE_HOSTNAME}" ]