FROM golang:1.24-bookworm

WORKDIR /build
COPY . ./
RUN CGO_LDFLAGS="-static" CGO_ENABLED=0 go build -a -tags netgo -ldflags '-w' -o yamdc ./cmd/yamdc

FROM alpine:3.13

COPY --from=0 /build/yamdc /bin/

RUN apk add --no-cache ffmpeg

ENTRYPOINT [ "/bin/yamdc" ]
