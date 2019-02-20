#!/bin/bash
#
# TODO: provisional 'build with params' notation
# types: s - string, freeform
#        c - choice (one-of), string
#        b - boolean (true/false)
#        
#-?s?DELAY?5
#-?c?SUITE?small|big|huge
#-?b?DEBUG?false
#
# all params are strings, so REST calls can encode as POST params
# eg., ".../some_job/?DELAY=5&SUITE=big&DEBUG=true
#  (these would override defaults first parsed from this file above, or set below)
DELAY=${DELAY:-"5"}
SUITE=${SUITE:-"small"}
DEBUG=${DEBUG:-"false"}
##

delay=${1:-"5"}

echo "workdir: ${PWD}"
ADIR="${BACILLUS_ARTFDIR}"
mkdir -p "${ADIR}"
echo "blah" >"${ADIR}/artifact.txt"

for i in $(seq 1 4); do
  echo "Doing some work (sleeping ${delay}). $i ..."
  sleep ${delay}
done

echo "--DONE--"
