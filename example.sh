#!/bin/bash
OPTS="${1:-''}"

## Example configuration launching the gofish webhook worker tool to handle
## multiple jobs/projects.
##  -- disparate projects, job environment variables, console
##  -- output and optional workdir cleanup.

## Listen for multiple endpoints. Build 'hkexsh' and 'gofish' projects
## on webhook notifications of push events.

## onPush_hkexsh_build - build https://blitter.com/gogs/RLabs/hkexsh
## onPush_gofish_nop - Just print pwd
## onPush_gofish_install - build and install https://blitter.com/gogs/Russtopia/gofish

gofish "${OPTS}" \
 onPush_hkexsh_build:workdir:FOO=bar,BAZ=buzz:"./hkexsh_pushbuild.sh" \
 onPush_gofish_nop:/tmp:GOFISH_REMOVE_WORKDIR=1,FOO=gofish_nop1:"/bin/bash -c pwd" \
 onPush_gofish_nop_nocleanup:/tmp:FOO=gofish_nop2:"/bin/bash -c ls gofish*" \
 onPush_gofish_install::FOO=gofish:"go install ."

