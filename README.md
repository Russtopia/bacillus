# gofish - A generic webhook work dispatcher


gofish listens for HTTP GET or POST events, executing specified actions on receipt of matching endpoint requests. Use it to respond to webhooks from SCM managers such as github, gitlab, gogs.io, etc. or from wget or curl requests made from git commit hooks, or anything else one can think of.


## Rationale

The goal of this project is to offer an extremely minimal Build Automation and
Continuous Integration (CI) system, with zero dependencies on large
frameworks, containers, etc. (though jobs invoked by gofish are free to use
whatever build/test frameworks are available).

Existing automated build/CI systems such as Jenkins, buildbot, concourse,
and so on are somewhat large and/or difficult to set up, and require
large external components (java, docker).


### Building and Installing

$ go install .

## Configuration

gofish, being a simple tool, has little configuration. The repository contains
two sample scripts:

* example.sh - launch gofish with a few demo endpoints
* workdir/hkexsh_pushbuild.sh - build job for an example external project

The second script above, workdir/hkexsh_pushbuild.sh, is intended to be triggered by either a git hook or a gogs.io webhook; the sample post-receive git hook script is also present within workdir/, but is active within the external git repo rather than being part of the gofish configuration itself.

In summary, to perform build/CI tasks with gofish, one should

* add a git/web hook to external git repositories and/or git repo web servers
* add job scripts to perform the intended tasks to some location known to gofish
* define endpoints, job workdir and job-specific env vars for each to pass
  to gofish (see example.sh)

## TODOs
* TODO: Add cmdline option to specify location of run.log (currently gofish launch dir)

## Example Run
Prerequisites: golang (for example hkexsh_pushbuild.sh build script as well as gofish itself)

[terminal A - CI server]
```
$ cd go/src/blitter.com/go/gofish
$ go install . && ./example.sh
```

[terminal B - client event test]

```
$ curl -s http://localhost:9990/blind/onPush_hkexsh_build
```

Observe execution on localhost:9990/runlog
