#!/bin/sh

os=""
arch=amd64
if [ $# -lt 1 ]; then
    echo "Usage: $0 version [ARCH [OS]] "
    exit
fi
if [ $# -gt 1 ]; then
    arch=$2
fi
if [ $# -gt 2 ]; then
    os=$3
fi

BUILD_VERSION=$1
BUILD_NAME="Tiger"
BUILD_TIME=$(date "+%F %T")
COMMIT_SHA1=$(git rev-parse HEAD)

doBuild(){
    echo "GOOS=${GOOS} GOARCH=${GOARCH} BUILD_VERSION=${BUILD_VERSION}  BUILD_NAME=${BUILD_NAME} BUILD_TIME=${BUILD_TIME} COMMIT_SHA1=${COMMIT_SHA1}"

    if [ "$GOOS" != "android" ]; then
        export GOOS=$1
        export GOARCH=$2
        ldflags="                                   \
            -s -w                                   \
            -X 'main.BuildVersion=${BUILD_VERSION}' \
            -X 'main.BuildTime=${BUILD_TIME}'       \
            -X 'main.CommitID=${COMMIT_SHA1}'       \
            -X 'main.BuildName=${BUILD_NAME}'       \
        "
        go build -o ./bin/ -ldflags "${ldflags}"
        go build -o ./bin/ -ldflags "${ldflags}" ./batch/batch.go
    else
        gomobile bind -target=android -o ./bin/bnrtc2.aar ./bnrtc2
    fi
    echo "===============================================  $GOOS $GOARCH build ok"
}

if [ "$os" != "" ]; then
    doBuild $os $arch
else
    doBuild "windows" $arch
    doBuild "linux" $arch
    doBuild "android" $arch
fi

