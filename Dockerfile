FROM golang:1.25-alpine AS builder

COPY . /src
WORKDIR /src

RUN GOPROXY=https://goproxy.cn make build

FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY --from=builder /src/bin /app
COPY --from=builder /src/configs /app/configs
COPY --from=builder /src/migrations /app/migrations

WORKDIR /app

EXPOSE 8000

CMD ["./app", "-conf", "/app/configs/config.yaml"]
