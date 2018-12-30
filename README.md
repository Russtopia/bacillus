# gofish - A generic webhook work dispatcher


gofish listens for webhook events, executing specified actions on receipt of matching POST endpoint requests.

### Building and Installing

$ grml install

or

$ go install .

### Testing

gofish can test itself by adding a webhook on the running system hosting the git repo, see
example1.sh in the top folder.

TODO: Example webhook and githook usage, native git hooks vs. gogs custom_hooks.
TODO: Add cmdline option to specify location of run.log (currently gofish launch dir)
TODO: Add API key checking to secure execution (via POST)
TODO: Add companion tools (console & web) to show recent run activity

