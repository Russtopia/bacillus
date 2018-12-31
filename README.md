# gofish - A generic webhook work dispatcher


gofish listens for webhook events, executing specified actions on receipt
of matching POST endpoint requests.

## Rationale

The goal of the project is to offer an extremely minimal Automated Build and
Continuous Integration (CI) system, with zero dependencies on large
frameworks, containers, etc. (though jobs invoked by gofish are free to use
whatever build/test frameworks are available).

Existing automated build/CI systems such as Jenkins, buildbot, concourse,
and so on are somewhat large and/or difficult to set up, or require
large external components (java, docker).

If one just wants something to respond to git commit hooks and/or
webhooks from other git web-hosting tools (gogs.io, gitlab, github, ...) and
in response run build/test/package steps, gofish is intended to be a much
simpler solution.


### Building and Installing

$ go install .

## Configuration

gofish, being a simple tool, has little configuration. The repository contains
two sample scripts:

* example.sh - launch gofish with a few demo endpoints
* workdir/hkexsh_pushbuild.sh - build job for an external project

The second script above, workdir/hkexsh_pushbuild.sh, is triggered by either
a git hook or a gogs.io webhook; the sample post-receive git hook script is
also present within workdir/, but is active within the external git repo
rather than being part of the gofish configuration itself.

In summary, to perform build/CI tasks with gofish, one should

* add a git/web hook to external git repositories and/or git repo web servers
* add job scripts to perform the intended tasks to some location known to gofish
* define endpoints, job workdir and job-specific env vars for each to pass
  to gofish (see example.sh)

## TODOs
* TODO: Add cmdline option to specify location of run.log (currently gofish launch dir)
* TODO: [?] Add companion tools (console & web) to show recent run activity
