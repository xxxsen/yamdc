#!/bin/bash

if [ "$(id -u)" != "0" ]; then 
    echo "should run as root"
    exit 1
fi 

apt update 
sudo apt-get install libdlib-dev libblas-dev libatlas-base-dev liblapack-dev libjpeg62-turbo-dev gfortran -y

