#!/bin/bash

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
