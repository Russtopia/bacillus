# bacillus - A minimalist Build Automation/CI service


bacillus listens for HTTP GET or POST events, executing specified actions on receipt of matching endpoint requests. Use it to respond to webhooks from SCM managers such as github, gitlab, gogs.io, etc. or from wget or curl requests made from git commit hooks, or anything else one can think of.


## Rationale

The goal of this project is to offer an extremely minimal Build Automation and
Continuous Integration (CI) system, with zero dependencies on large
frameworks, containers, etc.

Existing automated build/CI systems such as Jenkins, buildbot, concourse,
and so on are large and/or difficult to set up, and have many external dependencies.
bacillus is a single static binary with almost zero external configuration.

### Building and Installing

$ go install .

## Configuration

bacillus, being a simple tool, has little configuration. The repository contains
two sample scripts:

* example.sh - launch bacillus with a few demo endpoints
* workdir/hkexsh_pushbuild.sh - build job for an example external project

The second script above, workdir/hkexsh_pushbuild.sh, is intended to be triggered by either a git hook or a webhook such as those provided by repo managers such as gogs.io. The sample post-receive git hook script in workdir/ is meant to be placed within the external project's git repo, and as such is not technically part of the bacillus configuration itself.

In summary, to perform build/CI tasks with bacillus, one should

* add a git/web hook to external git repositories and/or git repo web servers
* add job scripts to perform the intended tasks to some location known to bacillus
* define endpoints, job workdir and job-specific env vars for each to pass
  to bacillus (see example.sh)

## TODOs
* TODO: Add cmdline option to specify location of run.log (currently bacillus launch dir)
* Screenshots
* Add a /jobstatus endpoint, showing a single-line status summary of all recent jobs w/status indicators (ie., brief version of /runlog and with only running and recently-completed jobs; or make /runlog do this and add a _full_log_ link at top to see complete logs)

## Example Run
Prerequisites: golang (for example hkexsh_pushbuild.sh build script as well as bacillus itself)

[terminal A - CI server]
```
$ cd go/src/blitter.com/go/bacillus
$ go install . && ./example.sh
```

[terminal B - client event test]

```
$ curl -s http://localhost:9990/blind/onPush_hkexsh_build
```

Observe execution on localhost:9990/runlog

## Controlling Jobs
* Endpoints in the /runlog view have a [>] beside them; you can launch jobs by clicking on these. The launched job status will appear in /runlog on the next page refresh (10s).
* When a job is launched, it has a [C] beside the launch entry. Clicking on this will cancel the job.
* To view a job's progress (its 'live console'), click on the jobID link.
* Running jobs show, in their live console page, the most recent output lines and, if the output grows large enough, a link to the full console output (at the moment) at the top. A coloured spinner/status bubble is at the bottom of the live console indicating the job's running or completion status.
* When a job completes, the spinner changes to an exit status code indicator and the live console stops refreshing.

