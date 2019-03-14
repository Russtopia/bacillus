#!/bin/bash

# Exit on error
set -e

export PATH=/usr/local/bin:/usr/bin:/usr/lib/ccache/bin:/bin

## NOTE jobs are responsible for creating their own ARTFDIR
mkdir -p "${BACILLUS_ARTFDIR}"

export GO111MODULE=auto
export GOPATH="${HOME}/go"
export PATH=$PATH:$GOPATH/bin

# GOCACHE will be phased out in v1.12. [github.com/golang/go/issues/26809]
export GOCACHE="${HOME}/.cache/go-build"

function stage {
  local _stage="${BACILLUS_WORKDIR}"/_stage
  
  echo -e "\n--STAGE: ${1}--\n"
  if [ ! -f ${_stage} ]; then
    echo -n "$1" >"${BACILLUS_WORKDIR}"/_stage
  else
    echo -n ":$1" >>"${BACILLUS_WORKDIR}"/_stage
  fi
}

stage "Fetch"
echo "workdir: ${BACILLUS_WORKDIR}"
echo "${PWD}"

git clone https://gogs.blitter.com/Russtopia/brevity
cd brevity

stage "Tests"
go test -v | tee tests.log

stage "Build"
go build .

stage "Artifacts"
tar czvf ${BACILLUS_ARTFDIR}/brevity.tgz [a-zA-Z0-9]*

echo "--Done--"
