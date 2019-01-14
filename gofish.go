// Package gofish is a Webhook listener that dispatches
// arbitrary commands on receipt of webhook events.
// Supported webhook event formats: gogs, (.. future)
//
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	appVer string = "v0.1"
	defKey string = "IAmABanana"
)

var (
	addrPort     string
	hookStd      string
	apiKey       string
	attachStdout bool
	jobHomeDir   string
)

// TODO: types for matching JSON events of
// supported webhooks: gogs.io, github, gitlab, ... ?

func consoleHandler(w http.ResponseWriter, r *http.Request) {
	if true {
		consoleLog, e := ioutil.ReadFile(fmt.Sprintf("%s", r.URL)[1:])
		if e != nil {
			w.Write([]byte(fmt.Sprintf("%s", e)))
			return
		}

		lines := strings.Split(string(consoleLog), "\n")
		tailL := 60
		l := len(lines) - tailL
		if l < 0 {
			l = 0
		}
		tail := lines[l:]
		if l > 0 {
			consoleLog = []byte("[click here for full log]\n...\n" +
				strings.Join(tail, "\n"))
		}

		w.Header().Set("Content-type", "text/html")
		io.WriteString(w, `
				<html>
				<head>
				<meta http-equiv="refresh" content="5">
				
				<style>
				  div {
				    position: fixed;
					left: 1em; bottom: 1em;
					font-family: monospace;
					margin: 1em;
					font-size: 1.5em;
					font-weight: normal; //bold;
					background: darkgreen;
					border: dotted 2px;
					border-radius: 1em;
				  }
				</style>

				<script>
				  bodyOrHtml = function() {
				    if ('scrollingElement' in document) {
					  return document.scrollingElement;
			  		}
					// Fallback for legacy browsers
					if (navigator.user-Agent.indexOf('WebKit') != -1) {
					  return document.body;
			  		}
					return document.documentElement;
				  }

				  scrollDown = function() {
				    console.log('FOo');
					setTimeout (function () {
					  bodyOrHtml().scrollTop = bodyOrHtml().scrollHeight;
	  				}, 5); // hack: delay due to most browsers' auto-scroll reset on page reload
				  }

				  appendSpinner = function() {
					var spinners = [
						"|/-\\",
						".oO@*",
						[">))'>"," >))'>","  >))'>","   >))'>","    >))'>","   <'((<","  <'((<"," <'((<"],
					];

					var el = document.createElement('div');
					document.body.appendChild(el);
					var spinner = spinners[0];
					
					(function(spinner,el) {
					  var i = 0;
					  setInterval(function() {
						el.innerHTML = spinner[i];
						i = (i + 1) % spinner.length;
					  }, 300);
					})(spinner,el);
				  }

				  window.onload = function() {
					appendSpinner();
					scrollDown(); //scrollTo(0,0);
				  }
				</script>

				</head>
				<body>
				`)

		io.WriteString(w, "<pre>")
		w.Write(consoleLog)
		io.WriteString(w, "</pre>")
		io.WriteString(w, "<pre>\n\n\n</pre>")
		// TODO: Consider a js spinner here, eg. <span id="spinner"></span>

		w.Write([]byte(fmt.Sprintln(r.URL)))
		io.WriteString(w, `
				</body>
				</html>
				`)
	} else {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "404 - Page Not Found")
	}
}

func main() {
	flag.StringVar(&addrPort, "a", ":9990", "[addr]:port on which to listen")
	flag.StringVar(&hookStd, "h", "blind", "hook type")
	flag.StringVar(&apiKey, "k", defKey, "API key")
	flag.BoolVar(&attachStdout, "s", false, "set to true to see worker stdout/err if running in terminal")
	flag.Parse()

	logfile, _ := os.Create("run.log")
	log.SetOutput(logfile)

	log.Printf("[gofish %s startup]\n", appVer)
	log.Printf("[listening on %s, type %s]\n", addrPort, hookStd)

	cmdMap := make(map[string]string)

	log.Printf("Registering handler for /runlog page...\n")
	http.HandleFunc("/runlog", func(w http.ResponseWriter, r *http.Request) {
		//header := w.Header()
		w.Header().Set("Content-type", "text/html")
		io.WriteString(w, `
				<html>
				<head>
				<meta http-equiv="refresh" content="10">
				</head>
				<body>
				`)
		//w.Write([]byte(fmt.Sprintf("%+v", header)))
		//w.Write([]byte("TODO: /runlog page\n"))
		io.WriteString(w, "<pre>")
		rl, _ := ioutil.ReadFile("run.log")
		w.Write(rl)
		io.WriteString(w, "</pre>")

		io.WriteString(w, `
				</body>
				</html>
				`)
	})

	jobHomeDir = "workdir"
	for _, e := range flag.Args() {
		fields := strings.Split(e, ":")
		tag := fields[0]
		var jobEnv []string
		var cmd string
		if len(fields) > 1 { /*&& fields[1] != "" {*/
			jobEnv = strings.Split(fields[1], ",")
		}
		if len(fields) > 2 && fields[2] != "" {
			cmd = fields[2]
		}

		cmdMap[tag] = cmd
		log.Printf("Registering handler for %s/%s [action %s]...\n", hookStd, tag, cmd)
		http.HandleFunc(fmt.Sprintf("/%s/%s", hookStd, tag),
			func(w http.ResponseWriter, r *http.Request) {
				go func() {
					cmdStrList := strings.Split(cmdMap[tag], " ")
					cmdArgs := []string{""}
					if len(cmdStrList) > 1 {
						cmdArgs = cmdStrList[1:]
					}
					c := exec.Command(cmdStrList[0], cmdArgs...)
					//c.Dir = execRoot

					// if jobHomeDir is set, set execution dir to
					// it and let user define whether or not to persist
					// WORKDIR via the GOFISH_WORKDIR env var.
					c.Dir, _ = filepath.Abs(jobHomeDir)

					//var terr error
					//var workDir string
					workDir, terr := ioutil.TempDir(c.Dir, "gofish_")
					jobID := strings.Split(workDir, "_")[1]
					indent, _ := strconv.ParseInt(jobID, 10, 64)
					indentStr := strings.Repeat("-", int(indent%8))
					log.Printf("%s[webhook %s{%s}: %s]\n", indentStr,
						tag, jobID, cmdMap[tag])
					if terr != nil {
						log.Printf("[ERROR creating workdir (%s) for event %s trigger.]\n", terr, tag)
					} else {
						var workerOutputPath string
						consoleFile := "console.out"
						workerOutputPath = workDir + "/" + consoleFile
						if attachStdout {
							c.Stdout = os.Stdout
							c.Stderr = os.Stderr
						} else {
							workerOutputFile, _ := os.Create(workerOutputPath)
							c.Stdout = workerOutputFile
							c.Stderr = workerOutputFile
						}

						c.Env = append(c.Env, fmt.Sprintf("USER=%s", os.Getenv("USER")))
						c.Env = append(c.Env, fmt.Sprintf("HOME=%s", os.Getenv("HOME")))
						c.Env = append(c.Env, fmt.Sprintf("GOFISH_JOBID=%s", jobID))
						c.Env = append(c.Env, fmt.Sprintf("GOFISH_JOBTAG=%s", tag))
						c.Env = append(c.Env, fmt.Sprintf("GOFISH_WORKDIR=%s", workDir))
						c.Env = append(c.Env, jobEnv...)

						cerr := c.Start()
						if cerr != nil {
							log.Printf("[exec.Cmd: %+v]\n", c)
							w.WriteHeader(500)
							w.Write([]byte("ERR"))
							log.Printf("%s[ERROR on event %s trigger.]\n", indentStr,
								tag)
						} else {
							w.Write([]byte("OK"))
							log.Printf("%s[event %s{%s} triggered. (workDir %s)]\n", indentStr,
								tag, jobID, workDir)
							log.Printf("%s[console log:<a href=\"%s/gofish_%s/"+consoleFile+"\">%s/gofish_%s/console.log</a>]\n", indentStr,
								jobHomeDir, jobID, jobHomeDir, jobID)
						}
						werr := c.Wait()
						if werr == nil {
							log.Printf("%s[webhook %s{%s} completed with status 0]\n", indentStr,
								tag, jobID)
						} else {
							log.Printf("%s[webhook %s{%s} completed with error %s]\n", indentStr,
								tag, jobID, werr)
						}
						if strings.Contains(strings.Join(c.Env, " "),
							"GOFISH_REMOVE_WORKDIR") {
							_ = os.RemoveAll(workDir)
						}
					}
				}()
			})
	}

	http.HandleFunc("/"+jobHomeDir+"/", consoleHandler)

	err := http.ListenAndServe(addrPort, nil)
	if err != nil {
		log.Fatal(err)
	}
}
