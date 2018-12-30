#!/bin/bash
export PATH=/usr/local/bin:/usr/bin:/usr/lib/ccache/bin:/bin

export GO111MODULE=auto
export GOPATH="${HOME}/go"
# GOCACHE will be phased out in v1.12. [github.com/golang/go/issues/26809]
export GOCACHE="${HOME}/.cache/go-build"

echo "workdir: ${GOFISH_WORKDIR}"
echo "---"
go env
echo "---"
echo "passed env:"
env
echo "---"

cd ${GOFISH_WORKDIR}
echo "curDir: $PWD"
rm -rf build
mkdir -p build
cd build
git clone https://blitter.com/gogs/RLabs/hkexsh
cd hkexsh
ls

make all
echo "--Done--"


