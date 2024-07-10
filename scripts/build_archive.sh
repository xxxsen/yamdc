#!/bin/bash

if [ "$#" != "3" ]; then 
    echo "$0"' $os $arch $filename'
    echo "example:"
    echo "-- $0 windows amd64 yamdc"
    echo "-- $0 linux amd64 yamdc"
    exit 1
fi 

os="$1"
arch="$2"
filename="$3"
output="${filename}-${os}-${arch}"
bname="$output"
if [ "$os" == "windows" ]; then 
    bname="$bname.exe"
fi 

CGO_LDFLAGS="-static" CGO_ENABLED=1 GOOS=${os} GOARCH=${arch} go build -a -tags netgo -ldflags '-w' -o ${bname} ./
tar -czf "$output.tar.gz" "$bname"
rm $bname