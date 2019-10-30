#!/bin/bash

#Exit on error
set -e

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
git clone https://gogs.blitter.com/RLabs/xs
cd xs
branch=$(git for-each-ref --sort=-committerdate --format='%(refname)' | head -n 1)
echo "Building most recent push on branch $branch"
git checkout "$branch"
ls

#stage "Stuff"
#
#for i in $(seq 1 20); do echo Doing stuff ${i}...; sleep 1; done

stage "Build"
make all

stage "Test(Authtoken)"
echo "Clearing test user $USER ~/.xs_id file ..."
rm -f ~/.xs_id
echo "Setting dummy authtoken in ~/.xs_id ..."
echo "localhost:asdfasdfasdf" >~/.xs_id
echo "Performing remote command on @localhost via authtoken login ..."
tokentest=$(timeout 10 xs -x "echo -n FOO" @localhost)
if [ "${tokentest}" != "FOO" ]; then
  echo "AUTHTOKEN LOGIN FAILED"
  exit 1
else
  echo "client cmd performed OK."
  unset tokentest
fi

stage "Test(S->C)"
echo "Testing secure copy from server -> client ..."
tmpdir=$$
mkdir -p /tmp/$tmpdir
cd /tmp/$tmpdir
xc @localhost:${BACILLUS_WORKDIR}/build/xs/cptest .
echo -n "Integrity check on copied files (sha1sum) ..."
sha1sum $(find cptest -type f | sort) >sc.sha1sum
diff sc.sha1sum ${BACILLUS_WORKDIR}/build/xs/cptest.sha1sum
stat=$?

cd -
rm -rf /tmp/$tmpdir
if [ $stat -eq "0" ]; then
  echo "OK."
else
  echo "FAILED!"
  exit $stat
fi

stage "Test(C->S)"
echo "TODO ..."

stage "Artifacts"
echo -n "Creating tarfile ..."
tar -cz --exclude=.git --exclude=cptest -f ${BACILLUS_ARTFDIR}/xs.tgz .

stage "Cleanup"
rm -f ~/.xs_id

echo
echo "--Done--"
