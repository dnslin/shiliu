ARG REGISTRY=docker.io
ARG GO_VERSION=1.24.10

FROM ${REGISTRY}/golang:${GO_VERSION}-alpine AS builder
RUN set -eux && sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories

WORKDIR /data/app

COPY go.mod go.sum ./
RUN export GOPROXY=https://goproxy.cn,direct && go mod download

COPY . .
RUN rm -rf /data/app/bin \
    && mkdir -p /data/app/bin \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ./bin/server ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ./bin/task ./cmd/task \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ./bin/migration ./cmd/migration

FROM ${REGISTRY}/alpine:3.20
RUN set -eux && sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories
RUN apk add --no-cache ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /data/app
COPY --from=builder /data/app/bin ./bin
COPY --from=builder /data/app/config ./config
COPY --from=builder /data/app/migrations ./migrations
RUN mkdir -p /data/app/storage

EXPOSE 8000
CMD ["./bin/server", "-conf", "config/prod.yml"]
