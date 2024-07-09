FROM golang:1.21

RUN apt update && apt install libdlib-dev libblas-dev libatlas-base-dev liblapack-dev libjpeg62-turbo-dev gfortran -y 
WORKDIR /build
COPY . ./
RUN CGO_LDFLAGS="-static" CGO_ENABLED=1 go build -a -tags netgo -ldflags '-w' -o yamdc ./

FROM alpine:3.12

COPY --from=0 /build/yamdc /bin/

RUN apk add --no-cache ffmpeg

ENTRYPOINT [ "/bin/yamdc" ]