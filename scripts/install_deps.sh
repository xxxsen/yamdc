#!/bin/bash

if [ "$#" != "1" ]; then
    echo "$0"' $os'
    echo "example:"
    echo "-- $0 linux"
    exit 1
fi 

os="$1"

if [ "$os" != "linux" ]; then 
    exit 0 
fi 

sudo apt update 
sudo apt-get install libdlib-dev libblas-dev libatlas-base-dev liblapack-dev libjpeg62-turbo-dev gfortran -y

