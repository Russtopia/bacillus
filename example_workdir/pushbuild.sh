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

#
# Note REPO_URI is implicitly exempted from sanitization due to _URI suffix
###
#-?s?REPO?xs?Repo Name
#-?s?REPO_URI?https://gogs.blitter.com/RLabs/xs?SCM URI of repo
#-?s?NOPATH_STRPARAM?./..//../Not_/../a_path?NOPATH_ prefix suppresses path sanitization (caveat emptor)
#-?s?STRPARAM?./..//.././../foo/bar?should end up as foo/bar
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

echo "NOPATH_ str param: ${NOPATH_STRPARAM}"
echo "regular str param (path-sanitized): ${STRPARAM}"

stage "Clone"
echo "curDir: $PWD"

rm -rf build
mkdir -p build
cd build
git clone ${REPO_URI}

## Hand off to project's build/test script
. ${REPO}/bacillus/${SCRIPT}

