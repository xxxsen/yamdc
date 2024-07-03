FROM golang:1.21

RUN apt update && apt install libdlib-dev libblas-dev libatlas-base-dev liblapack-dev libjpeg62-turbo-dev -y 
WORKDIR /build
COPY . ./
RUN CGO_ENABLED=1 go build -a -tags netgo -ldflags '-w' -o av-capture ./

FROM debian:bookworm

COPY --from=0 /build/av-capture /bin/

RUN apt update && apt install ffmpeg libdlib-dev libblas-dev libatlas-base-dev liblapack-dev libjpeg62-turbo-dev -y

ENTRYPOINT [ "/bin/av-capture" ]