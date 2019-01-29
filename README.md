# bacillɥs - A minimalist Build Automation/CI service


bacill&mu;s (**B**uild **A**utomation/**C**ontinuous **I**ntegration **L**ow-**L**inecount **ɥ**(micro)-**S**ervice) listens for HTTP GET or POST events, executing specified actions on receipt of matching endpoint requests. Use it to respond to webhooks from SCM managers such as github, gitlab, gogs.io, etc. or from wget or curl requests made from git commit hooks, or anything else one can think of.


## Rationale

The goal of this project is to offer an extremely minimal Build Automation and
Continuous Integration (CI) system, with zero dependencies on large
frameworks, containers, etc.

Existing automated build/CI systems such as Jenkins, buildbot, concourse, and so on are large and/or difficult to set up, with complex pre-built containers and/or entire bundles of packages, libraries and dependencies. bacill&mu;s is a single static binary with almost zero external configuration. Instead of taking one or (many) more evenings studying setup documentation, you can be up and running within minutes (assuming an existing golang installation on your server).

bacill&mu;s is language-agnostic: any script or binary that can be launched from a Linux shell can also be launched by bacill&mu;s.

### Building and Installing

```
[Login as user account that will run bacillus]
[Install recent version of Go, v1.11 or newer recommended]
$ git clone https://gogs.blitter.com/Russtopia/bacillus
$ cd bacillus
$ go install .
## .. finally, if you don't usually have $GOPATH/bin in your $PATH:
$ cp ./bacillus $PREFIX/bin  # .. where $PREFIX = $HOME, /usr/local, ... your choice
## .. Try it out!
$ ./bacillus_launch.sh # visit http://localhost:9990/
```


## Configuration

bacillus, being a simple tool, has little configuration. Almost all configuration is encapsulated in the tool's invocation command-line, and in the worker scripts themselves. Individual job configuration can be controlled by defining environment variables passed to each job within the invocation endpoint syntax.

## Structure

Sample installation tree

```
/home/account/
             /bacillus/                   (project tree)
                      /bacillus           (main binary)
                      /workdir/           (home of job scripts and running job workspaces)
                      /artifacts/         (where jobs place their 'artifacts' during/after run)
                      /images/            (image assets used by main binary)
                      bacillus_launch.sh  (example launch script with a few demo job endpoints)
```

## Tracking of Jobs

bacillus launches jobs as child processes, waiting on their exit and tagging their main stdout/stderr output, named 'console.out' within each worker's workspace (eg., *workdir/bacillus&lt;JOBID&gt;*). No external state or other meta-data is maintained, so there is no way to get out of sync with spawned jobs. If you kill the bacill&mu;s daemon, all currently-running jobs die too in standard UNIX fashion, unless jobs themselves detach via *nohup*.

The repository contains sample scripts:

* bacillus_launch.sh - launch bacillus with a few demo endpoints
* workdir/artifact.sh - simple example that just does 'work' for a short time and leaves artifacts
* workdir/hkexsh_pushbuild.sh - a slightly more realistic build job for an external project
* workdir/hkexsh_post-receive.sample - sample git post-receive hook used to trigger the above endpoint

In summary, to perform build/CI tasks with bacillus, one should

* add a git/web hook to external git repositories and/or git repo web servers
* add job scripts to workdir/ to perform the intended tasks
* define endpoints, jobOpts and jobEnv config for each to pass
  to bacillus (see bacillus_launch.sh)

## Storage

The design of bacillus follows the Unix tool philosophy: 'do one thing and do it well'. As such, scheduling of repeated jobs and reaping of old job workspaces/artifacts to save disk space, archiving etc. are left to external tools (consider using cron, anacron, rsync, etc.). An example cron job to reap old workspaces and artifacts is described within the 'bacillus_launch.sh' script.


## Larger Installations

To keep different categories of jobs logically separated and more manageable, consider grouping similar jobs together into the same launch script, and define a separate one for each such group (ie., daily builds vs. weekly builds vs. test jobs vs. git-triggered commit checks ...). Just change the endpoints specified in each copy of the launch script and the server port so each instance has its own web pages and bacill&mu;s daemon. They can all run within the same install tree if one wants, or separately; the tool does not enforce any specific policy.


## Example Run
Prerequisites: golang (for example hkexsh_pushbuild.sh build script as well as bacillus itself)

[terminal A - CI server]
```
$ cd <installation_dir>
$ ./bacillus_launch.sh
```

*Visit tool main page on localhost:9990/ (see bacillus_launch.sh to configure port)*

[terminal B - client event test]

```
$ curl -s http://localhost:9990/blind/onPush_hkexsh_build
```
