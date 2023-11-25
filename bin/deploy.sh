#!/usr/bin/env bash

set -Eeuo pipefail

echo Building
./bin/build.sh
echo Building Docker image
docker build --tag fuji:5000/game-freebies-bot:latest .
echo Pushing Docker image
version=$(IFS=. read -r a b c<<<"$(cat version.txt)";echo "$a.$b.$((c+1))")
echo $version > version.txt
docker tag fuji:5000/game-freebies-bot:latest fuji:5000/game-freebies-bot:$version
docker push fuji:5000/game-freebies-bot:latest
docker push fuji:5000/game-freebies-bot:$version
echo Deploying Helm chart
helm upgrade --install game-freebies-bot --set image.tag=$version  ./chart/