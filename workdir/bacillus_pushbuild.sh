#!/bin/bash
export PATH=/usr/local/bin:/usr/bin:/usr/lib/ccache/bin:/bin
echo "workdir: ${BACILLUS_WORKDIR}"
mkdir -p "${BACILLUS_ARTFDIR}"

export GO111MODULE=auto
export GOPATH="${HOME}/go"
# GOCACHE will be phased out in v1.12. [github.com/golang/go/issues/26809]
export GOCACHE="${HOME}/.cache/go-build"

echo "---"
go env
echo "---"
echo "passed env:"
env
echo "---"

#cd ${BACILLUS_WORKDIR}
echo "curDir: $PWD"
rm -rf build
mkdir -p build
cd build
git clone https://gogs.blitter.com/Russtopia/bacillus
cd bacillus

go build .

tar czvf ${BACILLUS_ARTFDIR}/bacillus.tgz .
echo "--Done--"
