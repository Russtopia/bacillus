# bacillɥs - A minimalist Build Automation/CI service


bacillus (**B**uild **A**utomation/**C**ontinuous **I**ntegration **L**ow-**L**inecount **ɥ**(micro)-**S**ervice) listens for HTTP GET or POST events, executing specified actions on receipt of matching endpoint requests. Use it to respond to webhooks from SCM managers such as github, gitlab, gogs.io, etc. or from wget or curl requests made from git commit hooks, or anything else one can think of.


## Rationale

The goal of this project is to offer an extremely minimal Build Automation and
Continuous Integration (CI) system, with zero dependencies on large
frameworks, containers, etc.

Existing automated build/CI systems such as Jenkins, buildbot, concourse,
and so on are large and/or difficult to set up, and have many external dependencies.
bacillus is a single static binary with almost zero external configuration.

### Building and Installing

```
[Login as user account that will run bacillus]
[Install recent version of Go, v1.11 or newer recommended]
$ git clone https://gogs.blitter.com/Russtopia/bacillus
$ go install .
```


## Configuration

bacillus, being a simple tool, has little configuration. Almost all configuration is encapsulated in the tool's invocation via command-line, and in the worker scripts themselves.

## Structure

Sample installation tree

```
/home/account/
             /bacillus/             (project tree)
                      /bacillus     (main binary)
                      /workdir/     (home of job scripts and running job workspaces)
                      /artifacts/   (where jobs place their 'artifacts' during/after run)
                      /images/      (image assets used by main binary)
```

## Tracking of Jobs

bacillus launches jobs as child processes, waiting on their exit and tagging their main stdout/stderr output, named 'console.out' within each worker's workspace (eg., workdir/bacillus_&lt;JOBID&gt;). No external state or other meta-data is maintained, so there is no way to get out of sync with spawned jobs.


The repository contains
two sample scripts:

* bacillus_launch.sh - launch bacillus with a few demo endpoints
* workdir/artifact.sh - simple example that just does 'work' for a short time and leaves artifacts
* workdir/hkexsh_pushbuild.sh - a slightly more realistic build job for an external project
* workdir/hkexsh_pushbuild.sh - sample git post-receive hook used to trigger the above endpoint

In summary, to perform build/CI tasks with bacillus, one should

* add a git/web hook to external git repositories and/or git repo web servers
* add job scripts to perform the intended tasks to some location known to bacillus
* define endpoints, job workdir and job-specific env vars for each to pass
  to bacillus (see bacillus_launch.sh)

## Storage

The design of bacillus follows the Unix tool philosophy: 'do one thing and do it well'. As such, scheduling of repeated jobs and reaping of old job workspaces/artifacts to save disk space, archiving etc. are left to external tools (consider cron, anacron, rsync, etc.). An example cron job to reap old workspaces and artifacts is described within the 'bacillus_launch.sh' script.


## Larger Installations

To keep different categories of jobs logically separated and more manageable, consider grouping similar jobs together and running them from separate instances of the 'bacillus_launch' script (ie., daily builds vs. weekly builds vs. test jobs ...). Just change the endpoints specified in each copy of the launch script and the server port so each instance has its own web pages. They can all run within the same install tree or separately, the tool does not enforce any specific policy.


## TODOs
* TODO: Add cmdline option to specify location of run.log (currently bacillus launch dir)
* Screenshots
* Devise a way to allow retention of 'X' recent builds per job (currently only time-based retention)

## Example Run
Prerequisites: golang (for example hkexsh_pushbuild.sh build script as well as bacillus itself)

[terminal A - CI server]
```
$ cd &lt;installation_dir&gt;
$ ./bacillus_launch.sh
```

[terminal B - client event test]

```
$ curl -s http://localhost:9990/blind/onPush_hkexsh_build
```

Visit tool main page on localhost:&lt;port&gt;/runlog (see bacillus_launch.sh for port)
