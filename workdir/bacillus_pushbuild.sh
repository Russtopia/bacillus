#!/bin/bash
export PATH=/usr/local/bin:/usr/bin:/usr/lib/ccache/bin:/bin
echo "workdir: ${BACILLUS_WORKDIR}"
mkdir -p "${BACILLUS_ARTFDIR}"

export GO111MODULE=auto
export GOPATH="${HOME}/go"
export PATH=$PATH:$GOPATH/bin

# GOCACHE will be phased out in v1.12. [github.com/golang/go/issues/26809]
export GOCACHE="${HOME}/.cache/go-build"

function stage {
  local _stage="${BACILLUS_WORKDIR}"/_stage
  
  if [ ! -f ${_stage} ]; then
    echo -n "$1" >"${BACILLUS_WORKDIR}"/_stage
  else
    echo -n ":$1" >>"${BACILLUS_WORKDIR}"/_stage
  fi
}


stage "Setup"

echo "---"
go env
echo "---"
echo "passed env:"
env
echo "---"

stage "Clean Workspace"

echo "curDir: $PWD"
rm -rf build

if [ ! -f $HOME/go/bin/grml ]; then
  stage "Tools"
  echo "Installing grml ..."
  go get github.com/desertbit/grml
  if [ ! -f $GOPATH/bin/grml ]; then
    echo "ERROR installing grml build tool."
    exit 1
fi

stage "Clone"

mkdir -p build
cd build
git clone https://gogs.blitter.com/Russtopia/bacillus
cd bacillus

stage "Tests"
grml tests

stage "Build"
grml app

stage "Artifacts"

tar czvf ${BACILLUS_ARTFDIR}/bacillus.tgz .
echo "--Done--"
