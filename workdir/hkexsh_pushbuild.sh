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

function stage {
  local _stage="${BACILLUS_WORKDIR}"/_stage
  
  echo -e "\n--STAGE: ${1}--\n"
  if [ ! -f ${_stage} ]; then
    echo -n "$1" >"${BACILLUS_WORKDIR}"/_stage
  else
    echo -n ":$1" >>"${BACILLUS_WORKDIR}"/_stage
  fi
}

#cd ${BACILLUS_WORKDIR}
stage "Clone"
echo "curDir: $PWD"
rm -rf build
mkdir -p build
cd build
git clone https://gogs.blitter.com/RLabs/hkexsh
cd hkexsh
ls

stage "Stuff"

for i in $(seq 1 20); do echo Doing stuff ${i}...; sleep 1; done

stage "Build"
make all

stage "Artifacts"
tar -czv --exclude=.git --exclude=cptest -f ${BACILLUS_ARTFDIR}/hkexsh.tgz .
echo "--Done--"

