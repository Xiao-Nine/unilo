FROM golang:1.25-alpine AS builder

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/unilo-server ./cmd/server

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /out/unilo-server /app/unilo-server

EXPOSE 8080

ENTRYPOINT ["/app/unilo-server"]
