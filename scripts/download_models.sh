#!/bin/bash

set -e

SAVEDIR="./models"

if [ "$#" != "0" ]; then 
    SAVEDIR="$1"
fi 

echo "downloading model files to dir:$SAVEDIR..."

if [ -d "$SAVEDIR" ]; then
    echo "DIR: $SAVEDIR exist, remove it first!"
    exit 0
fi 

mkdir "$SAVEDIR" -p 
curl https://github.com/Kagami/go-face-testdata/raw/master/models/shape_predictor_5_face_landmarks.dat -L -o "$SAVEDIR/shape_predictor_5_face_landmarks.dat"
curl https://github.com/Kagami/go-face-testdata/raw/master/models/dlib_face_recognition_resnet_model_v1.dat -L -o "$SAVEDIR/dlib_face_recognition_resnet_model_v1.dat"
curl https://github.com/Kagami/go-face-testdata/raw/master/models/mmod_human_face_detector.dat -L -o "$SAVEDIR/mmod_human_face_detector.dat"

echo "model files download succ"
