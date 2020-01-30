#!/bin/bash
##
# Example script allowing parameters to be set by user
# at launch time to run against any repo URI and
# to run different build/test/whatever scripts.
#
# For a simpler launch entry script see xs_pushbuild.sh
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

#-?s?REPO?xs?Repo Name
#-?s?REPO_URI?https://gogs.blitter.com/RLabs/xs?SCM URI of repo
#-?s?SCRIPT?ci_pushbuild.sh?Entry script within $REPO_URI/bacillus
# --
#
# all params are strings, so REST calls can encode as POST params
# eg., ".../some_job/?DELAY=5&SUITE=big&DEBUG=true
#  (these would override defaults first parsed from this file above, or set below)
#
# Defaults
# --
export REPO=${REPO:-"xs"}
export REPO_URI=${REPO_URI:-"https://gogs.blitter.com/RLabs/$REPO"}

stage "Clone"
echo "curDir: $PWD"

rm -rf build
mkdir -p build
cd build
git clone ${REPO_URI}

## Hand off to project's build/test script
. ${REPO}/bacillus/${SCRIPT}

