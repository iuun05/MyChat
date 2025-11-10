# 构建阶段
FROM golang:1.24-alpine AS builder

ENV GO111MODULE=on GOPROXY=https://goproxy.cn,direct CGO_ENABLED=0

WORKDIR /MyChat

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o mychat .

# 运行阶段
FROM alpine:latest

WORKDIR /MyChat

COPY --from=builder /MyChat/mychat .
COPY --from=builder /MyChat/config ./config
COPY --from=builder /MyChat/asset ./asset

EXPOSE 8080

CMD ["./mychat"]


