FROM golang:1.24-bookworm

WORKDIR /build
COPY . ./
RUN CGO_LDFLAGS="-static" CGO_ENABLED=0 go build -a -tags netgo -ldflags '-w' -o yamdc ./cmd/yamdc

FROM alpine:3.13

WORKDIR /app

COPY --from=0 /build/yamdc /app/yamdc

RUN apk add --no-cache ffmpeg

EXPOSE 8080

ENTRYPOINT ["/app/yamdc"]
CMD ["server", "--config", "/config/config.json"]
