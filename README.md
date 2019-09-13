# bacill&mu;s - A minimalist Build Automation/CI service


bacill&mu;s (**B**uild **A**utomation/**C**ontinuous **I**ntegration **L**ow-**L**inecount **&mu;**(micro)-**S**ervice) listens for HTTP GET or POST events, executing specified actions on receipt of matching endpoint requests. Use it to respond to webhooks from SCM managers such as github, gitlab, gogs.io, etc. or from wget or curl requests made from plain git commit hooks, or anything else one can think of.

![https://gogs.blitter.com/Russtopia/bacillus/raw/master/images/screenshot-1.png](https://gogs.blitter.com/Russtopia/bacillus/raw/master/images/screenshot-1.png)

## Rationale

The goal of this project is to offer an *extremely* minimal Build Automation (BA), Continuous Integration (CI) and Continuous Deployment (CD) system with zero dependencies on large frameworks, VMs or containers. It basically should run on a potato, if that potato can run binaries compiled with Go, without breaking a sweat.

Bacill&mu;s is language-agnostic. Any script or binary that can be launched from a shell can also be launched by bacill&mu;s. Bacill&mu;s doesn't force you to learn any flavour-of-the-week DSL (Domain-Specific Language).

Core features reflect those the author found essential while using, administering and customizing a more traditional ('butler-based') build automation system for a large dev team over multiple years. Experience showed that most of the 'extra stuff' was unnecessary and better achieved by utilizing common external tools.

Bacill&mu;s is a single static binary written in Go, with nearly zero external configuration. With so little to configure, one can be up and running within minutes: no containers, VMs, or DSLs (Domain-Specific Languages) required, though you can utilize those additional tools, if you wish, for your BA/CI/CD needs.

If you want a point-and-click build server that lets you make jobs without knowing what a shell or cron scheduler is, this probably isn't for you. But if you want a build server that serves as a launch point, has a minimal but useful web interface, and otherwise *stays out of your way*, read on.

## Building and Installing

* Install recent version of Go, v1.11 or newer recommended
* Login as user account that will run bacill&mu;s
```
$ git clone https://gogs.blitter.com/Russtopia/bacillus
$ cd bacillus
$ go build .
$ go install .
## .. finally, if you don't usually have $GOPATH/bin in your $PATH:
$ cp ./bacillus $PREFIX/bin  # .. where $PREFIX = $HOME, /usr/local, ... your choice
## .. Try it out!
$ ./bacillus_launch.sh
```
* Visit http://localhost:9990/


## Configuration

bacill&mu;s, being a simple tool, has little configuration. Almost all is encapsulated in the tool's invocation command-line and in the worker scripts themselves. Individual job configuration can be controlled by defining environment variables, either statically as passed to each job within the tool invocation via endpoint arguments, or dynamically, via job parameters encoded within each job script, which are parsed at job launch to present a form that the user can fill in prior to each run (parameterized jobs).

## Structure

Sample installation tree

```
/$HOME/
      bacillus/                       (project tree)
              bacillus                (main binary)
              workdir/                (home of job scripts and running job workspaces)
                     jobA.{sh,py,...} (job entry script for 'jobA')
              artifacts/              (where jobs place their 'artifacts' during/after run)
              images/                 (image assets used by main binary)
              bacillus_launch.sh      (example launch script with a few demo job endpoints)
```

## Tracking of Jobs

bacill&mu;s launches jobs as child processes, waiting on their exit and tagging their main stdout/stderr output, named 'console.out' within each worker's workspace (eg., *workdir/bacillus&lt;JOBID&gt;*). No external state or other meta-data is maintained, so there is no way to get out of sync with spawned jobs. If you kill the bacill&mu;s daemon, all currently-running jobs die too in standard UNIX fashion, unless jobs themselves detach via *nohup*.

The repository contains sample scripts and git hooks:

* bacillus_launch.sh - launch bacill&mu;s with a few demo endpoints
* example_workdir/artifact.sh - an example parameterized job that just does busy-work for a time and leaves artifacts
* example_workdir/hkexsh_pushbuild.sh - a slightly more realistic build job for an external project
* example_workdir/hkexsh_post-receive.sample - sample git post-receive hook used to trigger the above endpoint

In summary, to perform build/CI tasks with bacill&mu;s, one should

* add a git/web hook to external git repositories and/or git repo web servers
* add job scripts to workdir/ to perform the intended tasks
* define endpoints, jobOpts and jobEnv config for each to pass
  to bacill&mu;s (see bacillus_launch.sh)

Visual matching of job trigger and completion entries in the runlog can be indicated in various ways, controlled by the -i switch.
Valid values are ```[ none | indent | colour | both ]```.

## Job Environment

Jobs launched by bacill&mu;s get some default environment variables, which should be sufficient to bootstrap typical tasks:

* **USER** - user under which daemon runs
* **HOME** - home dir of user under which daemon runs
* **BACILLUS_JOBID** - numerical ID which is the tempDir() suffix added to workdir/ and artifacts/ dir
* **BACILLUS_JOBTAG** - the 'endpoint tag' specified in the launch arguments for the daemon binding a job to a run script
* **BACILLUS_ARTFDIR** - the *relative* path from the job's launch workdir to the directory where it should, if required, store artifacts (the job script is responsible for creating this dir before use)
* **NOTE**: All other env vars normally defined for **$USER**, as if logged in via shell, are also given to jobs.

A single run of a job will have workdir/ and artifacts/ dirs named ```bacillus_<jobOpts>_${BACILLUS_JOBTAG}_${BACILLUS_JOBID}```.

## Scheduling, Storage and Artifact Management

The design of bacill&mu;s follows the Unix tool philosophy: *do one thing and do it well*. As such, scheduling of repeated jobs and reaping of old job workspaces/artifacts to save disk space, archiving etc. are left to external tools (consider using cron, anacron, rsync, etc.). An example cron job to reap old workspaces and artifacts is given within the 'bacillus_launch.sh' script's comments.


## Larger Installations

To keep different categories of jobs logically separated and more manageable, consider grouping similar jobs together into the same launch script, and define a separate one for each such group (ie., daily builds vs. weekly builds vs. test jobs vs. git-triggered commit checks ...). Just change the endpoints specified in each copy of the launch script and the server port so each instance has its own web pages and bacill&mu;s daemon. They can all run within the same install tree if one wants, or separately; the tool does not enforce any specific policy.

## Access Control

If launched with the ```--auth``` option, bacill&mu;s gates access to all served content via HTTP basic auth. Over plain HTTP this is not secure, so one should run behind a reverse proxy (eg., define a subdomain 'bacillus.yourdomain.net' mapping to 'localhost:9990'). This protects the HTML UI for manually triggering jobs, viewing status and artifacts, and controlling soft and hard shutdowns, as well as git/SCM triggered endpoint actions to run jobs. See the AUTH options in the example bacillus_launch.sh.

For scripts and SCM hooks, to trigger an endpoint (job) via eg. ```wget``` or ```curl```, an initial login request must first be made to the bacill&mu;s server, which will reply with an HTTP basic auth challenge and the text ```Not logged in.```. Subsequent requests made with the proper username:password then can proceed. See bacillus_launch.sh for more information.

## Parameterized Builds

For jobs which may need user-settable parameters at each job invocation, parameters may be placed within
comments in the main job file; these are picked up by the tool to generate a web form the user can fill out
before launching the job. The basic syntax is

-?T?VAR?DEFVALUE?COMMENT

.. where T = [ s (string) | c (choice) | b (bool) ]
  choices are separated by a pipe (|) character.
  
Example

```
#-?s?DELAY?5?Delay in seconds
#-?c?SUITE?small|big|huge?Size of something
#-?b?DEBUG?1?
```

Param lines such as the above should start at column 0 alone on a line, after the comment character on a new line in the script (acceptable prefixes currently are '#', '/*' and '//')

A job containing the above would present a form with a text box, a dropdown list and a checkbox for each
of the job parameters. Each variable is added to the job's environment variables.

## Job Pipeline Views

There is support for a simple 'pipeline view' of the stages of running jobs. See
the 'stage' function in examples ```workdir/hkexsh_pushbuild.sh``` and ```workdir/bacillus-pushbuild.sh```. Stages up to and including the running stage will be displayed at the end of the running job's entry in the runlog view.

## Example Run
Prerequisites: golang (for example hkexsh_pushbuild.sh build script as well as bacill&mu;s itself)

[terminal A - CI server]
```
$ cd <installation_dir>
$ ./bacillus_launch.sh
```

*Visit tool main page on localhost:9990/ (see bacillus_launch.sh to configure port)*

[terminal B - client event test]

```
$ curl -s http://localhost:9990/onPush-hkexsh-build
```

If ```--auth``` is used, the curl request will require credentials to activate the job endpoint, eg:

```
$ curl -s --netrc-file hooks/auth.txt http://localhost:9990/onPush-bacillus-build
```

Likewise, a web browser will present a user/pass authorization popup before allowing access to the web interface.


## Code Size
```
$ sloc *.go
  Languages  Files  Code  Comment  Blank  Total  CodeLns
      Total      3   806      744    124   1674   100.0%
         Go      3   806      744    124   1674   100.0%
```

#### ?! This isn't a microservice! It isn't RESTful!

Go away, ya bloody pedant.
