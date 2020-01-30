#!/bin/bash
##
# Example script to clone a specific hardcoded repo and chain to its
# build/test/whatever script.
#
# For a similar, parameterized example that can run from a variety of repos
# and launch different scripts, see pushbuild.sh in this directory.
##

#Exit on error
set -e

#################################################
function stage {
  local _stage="${BACILLUS_WORKDIR}"/_stage
  
  echo -e "\n--STAGE: ${1}--\n"
  if [ ! -f ${_stage} ]; then
    echo -n "$1" >"${BACILLUS_WORKDIR}"/_stage
  else
    echo -n ":$1" >>"${BACILLUS_WORKDIR}"/_stage
  fi
}
#################################################

export REPO=xs
export REPO_URI=https://gogs.blitter.com/RLabs/${REPO}

stage "Clone"
echo "curDir: $PWD"

rm -rf build
mkdir -p build
cd build
git clone ${REPO_URI}

## Hand off to project's build/test script
. ${REPO}/bacillus/ci_pushbuild.sh

