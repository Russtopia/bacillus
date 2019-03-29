#!/bin/bash
#
# Parameterized build syntax
# --
# types: s - string, freeform
#        c - choice (one-of), string
#        b - boolean (true/false)
#        
#-?s?DELAY?5?(in seconds)
#-?c?SUITE?small|big|huge?heap size
#-?b?DEBUG?1?Keep debug symbols
#-?b?Detonator?disabled
# --
#
# all params are strings, so REST calls can encode as POST params
# eg., ".../some_job/?DELAY=5&SUITE=big&DEBUG=true
#  (these would override defaults first parsed from this file above, or set below)
#
# Defaults
# --
DELAY=${DELAY:-"5"}
SUITE=${SUITE:-"small"}
DEBUG=${DEBUG:-"false"}
##

function stage {
  local _stage="${BACILLUS_WORKDIR}"/_stage
  
  echo -e "\n--STAGE: ${1}--\n"
  if [ ! -f ${_stage} ]; then
    echo -n "$1" >"${BACILLUS_WORKDIR}"/_stage
  else
    echo -n ":$1" >>"${BACILLUS_WORKDIR}"/_stage
  fi
}

delay=${1:-"5"}

stage "Setup"
echo "workdir: ${PWD}"
env

stage "Busywork"
for i in $(seq 1 4); do
  echo "Doing some work (sleeping ${delay}). $i ..."
  sleep ${delay}
done

stage "S1"
for i in $(seq 1 4); do
  echo "Doing some work (sleeping ${delay}). $i ..."
  sleep ${delay}
done

stage "S2"
for i in $(seq 1 4); do
  echo "Doing some work (sleeping ${delay}). $i ..."
  sleep ${delay}
done

stage "S3"
for i in $(seq 1 4); do
  echo "Doing some work (sleeping ${delay}). $i ..."
  sleep ${delay}
done

stage "S4"
for i in $(seq 1 4); do
  echo "Doing some work (sleeping ${delay}). $i ..."
  sleep ${delay}
done

stage "Artifacts"
ADIR="${BACILLUS_ARTFDIR}"
mkdir -p "${ADIR}"
echo "blah" >"${ADIR}/artifact.txt"

stage "Post Processing"
for i in $(seq 1 4); do
  echo "Doing some work (sleeping ${delay}). $i ..."
  sleep ${delay}
done


echo "--DONE--"
