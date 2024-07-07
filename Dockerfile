FROM golang:1.21

RUN apt update && apt install libdlib-dev libblas-dev libatlas-base-dev liblapack-dev libjpeg62-turbo-dev gfortran -y 
WORKDIR /build
COPY . ./
RUN CGO_LDFLAGS="-static" CGO_ENABLED=1 go build -a -tags netgo -ldflags '-w' -o av-capture ./

FROM alpine:3.12

COPY --from=0 /build/av-capture /bin/

RUN apk add ffmpeg

ENTRYPOINT [ "/bin/av-capture" ]