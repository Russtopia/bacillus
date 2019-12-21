#!/bin/bash

##
## Example bacillus launch script to trigger
## work on a few demonstration webhook endpoints.
##
## Syntax of an endpoint:
##  endpoint:jobOpts:EVAR1=val1,EVAR2=val2[,...,EVAR<n>=val<n>]:cmd
##
## Currently _jobOpts_ is user-defined, and is not interpreted by bacillus
## at all. It is merely inserted into the tempDir() generated for each job
## instance, to allow external tools to do things based on the jobOpts
## of each (such as an external tempDir and artifact dir reaper,
## to clean up various jobs based on differing schedules).
##
## bacillus launches each worker within its own start location,
##
## workdir/bacillus_<JOBID>
##
## and sets $BACILLUS_WORKDIR to this path for use by the worker.
## Each endpoint script should use $BACILLUS_WORKDIR to do its work and not
## pollute other filesystem locations; the tool does not prevent one from
## doing so (obviously, run the tool as an unprivileged user!)
##
## Following the _endpoint_ and _jobOpts_ fields is a comma-delimited list
## of zero or more user-defined environment variables, allowing unique job
## parameters to be passed as part of the final _cmd_ field's shell environment
## (of course, command-line arguments may also be specified in the _cmd_ field).
##
## The final _cmd_ field is a standard bash shell command line.
##
## the first of which is the command itself (usually a user-defined script)
## followed by zero or more arguments. NOTE that _cmd_ is launched via
## Go's exec API which uses the POSIX exec() semantics (ie., it is not
## launched via a shell) so bash-style variable expansions do NOT occur here.
## Use the preceding EVAR assignments to pass in values which are not known
## ahead of job invocation.
##

## SAMPLE ENDPOINT WORKERS
##
## onPush_hkexsh_build:
##   -build Go hkexsh project within workdir/, artifacts will
##    be in $BACILLUS_WORKDIR (bacillus_<nnnn>)
##    final build tree artifacts will be in $BACILLUS_ARTFDIR
##    job script is workdir/hkexsh_pushbuild.sh
##
## onPush_bacillus_env:
##   -Output the job's shell environment, calling 'env' via bash
##    and create a trivial artifact file in $BACILLUS_ARTFDIR
##
## bacillus will log worker activity to run.log in the current directory
## (wherever bacillus was launched from).

## Invoking each trigger using wget
# $ wget 127.0.0.1:9990/blind/onPush_hkexsh_build
# $ wget 127.0.0.1:9990/blind/onPush_bacillus_env

## Invoking via curl (in future this is required for any hook type
## beyond 'blind', as github, gitlab, gogs.io etc. send JSON via POST)
# $ curl -s -X POST -d [json] localhost:9990/gogs/<jobTag>

PORT=${PORT:-9990}
AUTH=${AUTH:-false}
AUTH_USER=${AUTH_USER:-"foo"}
AUTH_PASS=${AUTH_PASS:-"bar"}
RUNLOG_LIVE_VIEW_LINES=${RUNLOG_LIVE_VIEW_LINES:-30}
F=${F:-true} # show pipeline stages on finished job runlog entries
JOBLIMIT=${JOBLIMIT:-"8"} # default, override to set
DEMO=${DEMO:-false}
WORKDIR=${WORKDIR:-"example_workdir"}

## If better log rotation is desired, use logrotate.
if [ -e run"${PORT}".log ] && [ ${1:-"n"} == "-r" ]; then
  echo "Rolling previous log run${PORT}.log to run${PORT}.log.bak"
  mv -f run"${PORT}".log run"${PORT}".log.bak
fi

## Tagging jobs for periodic reaping
##
## Note the jobOpts 'kD','kF' in this example are completely user-defined:
## bacillus merely propagates these into work- and artifact directory names
## to allow external tools to filter them. For example, here they stand for
## 'keep Day' and 'keep Forever', and a cron job could use the tags to
## reap old job dirs:
##
## PATH=/bin:/usr/bin:/usr/local/bin:$HOME/bin/bacillus
##
##* * * * 1 rm -rf $(find $HOME/bacillus/artifacts $HOME/bacillus/workdir -type d -mmin +1440 -name "bacillus_kD*")
##* * 1 * * rm -rf $(find $HOME/bacillus/artifacts $HOME/bacillus/workdir -type d -mmin +10080 -name "bacillus_kW*")
##0 * * * * curl -s --netrc-file $HOME/bacillus-auth.txt https://bacillus.blitter.com/onPush-xs-build >/dev/null 2>&1
##

bacillus -D="${DEMO}"\
 -w="${WORKDIR}"\
 -F="${F}"\
 -jl="${JOBLIMIT}"\
 -a=:"${PORT}"\
 -rl="${RUNLOG_LIVE_VIEW_LINES}" \
 -auth=${AUTH} -u=${AUTH_USER} -p=${AUTH_PASS} \
 onPush-bacillus-build:kD::"../bacillus_pushbuild.sh" \
 onPush-xs-build:kD:FOO=bar,BAZ=buzz:"../xs_pushbuild.sh" \
 onPush-bacillus-artifact:kW:BACILLUS_FOO=foo,BACILLUS_BAR=bar:"../artifact.sh" \
 onPush-brevity-build:kD::"../brevity_pushbuild.sh"
