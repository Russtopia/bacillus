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
git clone https://gogs.blitter.com/RLabs/hkexsh
cd hkexsh
ls

for i in $(seq 1 20); do echo Doing stuff ${i}...; sleep 1; done

make all
echo "--Done--"
## For demonstration, succeed or fail based on BACILLUS_JOBID's evenness
if [ $(( ${BACILLUS_JOBID#0} % 2 )) -eq 0 ]; then
  echo "Succeeded."
  echo "artifacts for a successful run should go here." >${BACILLUS_ARTFDIR}/artifacts.txt
  exit 0
else
  echo "FAILED!"
  echo "(Not really, just simulating due to odd-numbered BACILLUS_JOBID:$BACILLUS_JOBID)"
  echo "Even for a failed run, artifacts might be placed here." >${BACILLUS_ARTFDIR}/failed.txt
  exit 252
fi
