#!/bin/bash

##
## Example gofish launch script to trigger
## work on a few demonstration webhook endpoints.
##
## Syntax of an endpoint:
##  endpoint:jobDir:EVAR1=val1,EVAR2=val2[,...,EVAR<n>=val<n>]:cmd
##
## gofish launches each worker within its own start location (eg., if one
## starts gofish within /opt/gofish/, each worker starts off there),
## but creates a per-job unique dir beneath _jobDir_ named gofish_<nnnn>
## and sets GOFISH_WORKDIR to this path for use by the worker.
## Each endpoint script should use GOFISH_WORKDIR to do its work and not
## pollute other filesystem locations.
##
## Following the _jobDir_ field is a comma-delimited list of zero or more
## user-defined environment variables, allowing unique job parameters to
## be passed as part of the final _cmd_ field's shell environment.
##
## The final _cmd_ field is a set of one or more space-delimited tokens,
## the first of which is the command itself (usually a user-defined script)
## followed by zero or more arguments. NOTE that _cmd_ is launched via
## Go's exec API which uses the POSIX exec() semantics (ie., it is not
## launched via a shell) so bash-style variable expansions do NOT occur here.
## Use the preceding EVAR assignments to pass in values to your scripts.
##

## SAMPLE ENDPOINT WORKERS
##
## onPush_hkexsh_build:
##   -build Go hkexsh project within workdir/, artifacts will
##    be in $GOFISH_WORKDIR (gofish_<nnnn>)
##    job script is workdir/hkexsh_pushbuild.sh
##
## onPush_gofish_env:
##   -Output the job's shell environment, calling 'env' via bash
##
## gofish will log worker activity to run.log in the current directory
## (wherever gofish was launched from).
## TODO: allow specifying location of run.log via an option

## Invoking each trigger using wget
# $ wget 127.0.0.1:9990/blind/onPush_hkexsh_build
# $ wget 127.0.0.1:9990/blind/onPush_gofish_nop

## Invoking via curl (in future this is required for any hook type
## beyond 'blind', as github, gitlab, gogs.io etc. send JSON via POST)
# $ curl -s -X POST -d [json] localhost:9990/gogs/<jobTag>

OPTS=${1:-''}

if [ -e run.log ]; then
  echo "Rolling previous log to run.log.bak"
  mv -f run.log run.log.bak
fi

gofish "${OPTS}" \
 onPush_hkexsh_build:FOO=bar,BAZ=buzz:"./hkexsh_pushbuild.sh" \
 onPush_gofish_env:GOFISH_FOO=foo,GOFISH_BAR=bar:"/bin/bash -c env"
