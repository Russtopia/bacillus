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
git clone https://gogs.blitter.com/RLabs/hkexsh
cd hkexsh
ls

for i in $(seq 1 20); do echo Doing stuff ${i}...; sleep 1; done

make all
echo "--Done--"
## For demonstration, succeed or fail based on GOFISH_JOBID's evenness
if [ $(( ${GOFISH_JOBID#0} % 2 )) -eq 0 ]; then
  echo "Succeeded."
  exit 0
else
  echo "FAILED!"
  exit 1
fi
