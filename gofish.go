// Package gofish is a Webhook listener that dispatches
// arbitrary commands on receipt of webhook events.
// Supported webhook event formats: gogs, (.. future)
//
package main

import (
	"context"
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
	"syscall"
)

const (
	appVer string = "v0.1"
	defKey string = "IAmABanana" //TODO: rudimentary API guarding
)

var (
	addrPort        string
	hookStd         string
	apiKey          string
	attachStdout    bool
	jobHomeDir      string
	runLogTailLines int

	jobCancellers map[string]func()
)

func getRefreshJS(stat rune) string {
	if stat == 'r' {
		return `<meta http-equiv="refresh" content="5">`
	} else {
		return ``
	}
}

func getSpinnerJS(stat rune, codeColor, statWord string) string {
	if stat == 'r' {
		return `<script>
////////////////////////
appendSpinner = function() {
  var spinners = [
    "|/-\\",
    ".oO@*",
    [">))'>"," >))'>","  >))'>","   >))'>","    >))'>","   <'((<","  <'((<"," <'((<"],
  ];

  var el = document.createElement('div');
  el.setAttribute('id', 'spinner');
  document.body.appendChild(el);
  el.innerHTML = '.';
  var spinner = spinners[0];
  
  (function(spinner,el) {
    var i = 0;
    setInterval(function() {
      el.innerHTML = spinner[i];
      i = (i + 1) % spinner.length;
    }, 300);
  })(spinner,el);
}
////////////////////////
</script>`
	} else {
		return `<script>
		////////////////////////
appendSpinner = function() {
  var el = document.createElement('div');
  el.setAttribute('id', '` + codeColor + `');
  el.innerHTML = '` + statWord + `';
  document.body.appendChild(el);
}
////////////////////////
</script>`
	}
}

func getStyleCSS() string {
	return `
  <style>
    #spinner {
      position: fixed;
      right: 1em; bottom: 1.5em;
      font-family: monospace;
      margin: 1em;
      padding: 0.2em;
      font-size: 1.5em;
      font-weight: normal; //bold;
      background: skyblue;
      border: dotted 2px;
      border-radius: 1em;
    }
	
    #finOKMarker {
      position: fixed;
      right: 1em; bottom: 1.5em;
      font-family: monospace;
      margin: 1em;
      padding: 0.2em;
      font-size: 1.5em;
      font-weight: normal;
      background: lightgreen;
      border: dotted 2px;
      border-radius: 1em;
    }
	
    #finErrMarker {
      position: fixed;
      right: 1em; bottom: 1.5em;
      font-family: monospace;
      margin: 1em;
      padding: 0.2em;
      font-size: 1.5em;
      font-weight: bold;
      background: red;
      border: dotted 2px;
      border-radius: 1em;
    }
	
    //#stat {
    //  display: none;
    //}
  </style>
  `
}

func getCompatJS() string {
	return `
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
      setTimeout (function () {
        bodyOrHtml().scrollTop = bodyOrHtml().scrollHeight;
      }, 5); // hack: delay due to most browsers' auto-scroll reset on page reload
    }
    </script>
	`
}

// TODO: types for matching JSON events of
// supported webhooks: gogs.io, github, gitlab, ... ?
// For now, the 'blind' endpoint is the only one supported,
// meaning the request can't communicate any extra data to the
// job invocation in a GET or POST request.

func fullConsoleHandler(w http.ResponseWriter, r *http.Request) {
	consoleLog, e := ioutil.ReadFile(strings.Replace(fmt.Sprintf("%s", r.URL)[1:], "/fullconsole", "", 1))
	if e != nil {
		w.Write([]byte(fmt.Sprintf("%s", e)))
		return
	}
	w.Header().Set("Content-type", "text/plain")
	w.Write(consoleLog)
}

func consoleHandler(w http.ResponseWriter, r *http.Request) {
	//if true {

	// Read file from URL, removing leading / as workdir is rel to us
	consoleLog, e := ioutil.ReadFile(fmt.Sprintf("%s", r.URL)[1:])
	if e != nil {
		w.Write([]byte(fmt.Sprintf("%s", e)))
		return
	}

	lines := strings.Split(string(consoleLog), "\n")
	// Prevent log output from creating huge web pages.
	// TODO: Add logic to link to full console log on first line
	tailL := 34
	l := len(lines) - tailL
	if l < 0 {
		l = 0
	}
	consStat := lines[0]
	fullConsLink := lines[1]
	var tail []string

	//TODO: analyze line 1, if status == 'f' and exitCode == '000' then suppress spinner
	// and reload timer; if exitCode != '000', consider red static '!' in place of spinner
	// (and still no reload)
	var stat rune
	var code int //TODO: use to determine red failure marker?
	n, e := fmt.Sscanf(consStat, "[%c %03d]", &stat, &code)
	_ = n
	if l > 0 {
		tail = lines[len(lines)-tailL:]
		//consoleLog = []byte(consStat + "\n" + fullConsLink + "\n" +
		//	strings.Join(tail, "\n"))
		_ = fullConsLink
		consoleLog = []byte("<a href=\"" + fullConsLink + "\">full log</a>\n" + strings.Join(tail, "\n"))
	} else {
		tail = lines[2:]
		//consoleLog = []byte(consStat + "\n\n" + strings.Join(tail, "\n"))
		consoleLog = []byte(strings.Join(tail, "\n"))
	}

	var codeColor string
	var statWord string
	if code != 0 {
		codeColor = "finErrMarker"
		statWord = fmt.Sprintf("E:%d", code)
	} else {
		codeColor = "finOKMarker"
		statWord = "Done"
	}

	w.Header().Set("Content-type", "text/html")
	io.WriteString(w, `
<html>
<head>
`+getRefreshJS(stat)+
		getStyleCSS()+
		getCompatJS()+
		getSpinnerJS(stat, codeColor, statWord)+
		`
  <script>
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

	w.Write([]byte(fmt.Sprintln(r.URL)))
	io.WriteString(w, `
</body>
</html>
`)
}

func launchJobListener(mainCtx context.Context, tag string, jobEnv []string, cmdMap map[string]string) {
	http.HandleFunc(fmt.Sprintf("/%s/%s", hookStd, tag),
		func(w http.ResponseWriter, r *http.Request) {
			go func() {
				//log.Printf("URL params:%+v\n", r.URL.Query())
				//if r.URL.Query()["p1"] != nil {
				//	log.Printf("p1:%s", r.URL.Query()["p1"][0])
				//}
				cmdStrList := strings.Split(cmdMap[tag], " ")
				cmdArgs := []string{""}
				if len(cmdStrList) > 1 {
					cmdArgs = cmdStrList[1:]
				}
				cmdCancelCtx, cmdCancelFunc := context.WithCancel(mainCtx)
				_ = cmdCancelFunc // TODO: call if user cancels job
				c := exec.CommandContext(cmdCancelCtx, cmdStrList[0], cmdArgs...)
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
				indentStr := strings.Repeat("-", int(indent%8)+1)
				log.Printf("%s[webhook %s{%s}: %s]\n", indentStr,
					tag, jobID, cmdMap[tag])
				if terr != nil {
					log.Printf("[ERROR creating workdir (%s) for event %s trigger.]\n", terr, tag)
				} else {
					var workerOutputPath string
					var workerOutputFile *os.File
					consoleFName := "console.out"
					workerOutputPath = workDir + "/" + consoleFName
					workerOutputRelPath := fmt.Sprintf("%s/gofish_%s/%s", jobHomeDir, jobID, consoleFName)
					if attachStdout {
						c.Stdout = os.Stdout
						c.Stderr = os.Stderr
					} else {
						workerOutputFile, _ = os.Create(workerOutputPath)
						c.Stdout = workerOutputFile
						c.Stderr = workerOutputFile
					}

					c.Env = append(c.Env, fmt.Sprintf("USER=%s", os.Getenv("USER")))
					c.Env = append(c.Env, fmt.Sprintf("HOME=%s", os.Getenv("HOME")))
					c.Env = append(c.Env, fmt.Sprintf("GOFISH_JOBID=%s", jobID))
					c.Env = append(c.Env, fmt.Sprintf("GOFISH_JOBTAG=%s", tag))
					c.Env = append(c.Env, fmt.Sprintf("GOFISH_WORKDIR=%s", workDir))
					c.Env = append(c.Env, jobEnv...)

					// Job output status is encoded in first line of output log.
					// [1 2]
					//  1: state: r = running f = finished
					//  2: completion status: <n> = exit status, 0 = success; else failure
					//     status uses UNIX shell exit status convention (base 10 0-255))
					_, err := fmt.Fprintf(c.Stdout, "[r 255]\n")
					_, err = fmt.Fprintf(c.Stdout, "%s\n", strings.Replace(workerOutputRelPath, "workdir/", "/workdir/fullconsole/", 1))
					if err != nil {
						log.Fatal(err)
					}

					cerr := c.Start()
					if cerr != nil {
						log.Printf("[exec.Cmd: %+v]\n", c)
						w.WriteHeader(500)
						w.Write([]byte("ERR"))
						log.Printf("%s[ERROR on event %s trigger.]\n", indentStr,
							tag)
					} else {
						jobCancellers[jobID] = cmdCancelFunc
						w.Write([]byte("OK"))
						log.Printf("%s<a href='/cancel/%s'>[C]</a>[event %s{%s} triggered. (workDir %s)]\n", indentStr,
							jobID,
							tag, jobID, workDir)
						log.Printf("%s[console log:<a href=\"%s\">%s</a>]\n", indentStr,
							workerOutputRelPath, workerOutputRelPath)

						// Spawn handler for /cancel/<jobID>
						http.HandleFunc(fmt.Sprintf("/cancel/%s", jobID),
							func(w http.ResponseWriter, r *http.Request) {
								if jobCancellers[jobID] != nil {
									delete(jobCancellers, jobID)
									jobCancellers[jobID]()
								}
								w.Write([]byte(fmt.Sprintf("Cancelled %s", jobID)))
							})
					}
					werr := c.Wait()
					if jobCancellers[jobID] != nil {
						jobCancellers[jobID]()
					}
					delete(jobCancellers, jobID)

					if werr, ok := werr.(*exec.ExitError); ok {
						// The program has exited with an exit code != 0

						// This works on both Unix and Windows. Although package
						// syscall is generally platform dependent, WaitStatus is
						// defined for both Unix and Windows and in both cases has
						// an ExitStatus() method with the same signature.
						var exitStatus uint32
						if status, ok := werr.Sys().(syscall.WaitStatus); ok {
							exitStatus = uint32(status.ExitStatus())
							// exec.Cmd automatically closes its files on exit, so we need to
							// reopen here to write the status at offset 0
							workerOutputFile, _ = os.OpenFile(workerOutputPath, os.O_RDWR, 0777)
							fmt.Fprintf(workerOutputFile, "[f %03d]", exitStatus)
							//log.Print(c.Stderr /*stdErrBuffer*/)
							log.Printf("Exit Status: %d\n", int32(exitStatus)) //#
						}
					} else {
						// exec.Cmd automatically closes its files on exit, so we need to
						// reopen here to write the status at offset 0
						workerOutputFile, _ = os.OpenFile(workerOutputPath, os.O_RDWR, 0777)
						fmt.Fprintf(workerOutputFile, "[f %03d]", 0)
						//workerOutputFile.WriteAt([]byte(fmt.Sprintf("[f %03d]", 0)), 0)
					}

					// TODO: exitStatus output to.. job.status ? (int exitStatus)
					// TODO: console log endpoint check for existence of job.status;

					if werr == nil {
						log.Printf("%s[webhook %s{%s} completed with status 0]\n", indentStr,
							tag, jobID)
					} else {
						log.Printf("%s[webhook <a href=\"%s\">%s{%s}</a> completed with error %s]\n", indentStr,
							workerOutputRelPath, tag, jobID, werr)
					}
					if strings.Contains(strings.Join(c.Env, " "),
						"GOFISH_REMOVE_WORKDIR") {
						_ = os.RemoveAll(workDir)
					}
				}
			}()
		})
}

func main() {
	flag.StringVar(&addrPort, "a", ":9990", "[addr]:port on which to listen")
	flag.StringVar(&hookStd, "h", "blind", "hook type")
	flag.StringVar(&apiKey, "k", defKey, "API key")
	flag.IntVar(&runLogTailLines, "rl", 32, "Scroll length of runlog (set to 0 for no limit)")
	flag.BoolVar(&attachStdout, "s", false, "set to true to see worker stdout/err if running in terminal")
	flag.Parse()

	mainCtx := context.Background()
	jobCancellers = make(map[string]func())

	logfile, _ := os.Create("run.log")
	log.SetOutput(logfile)

	log.Printf("[gofish %s startup]\n", appVer)
	log.Printf("[listening on %s, type %s]\n", addrPort, hookStd)

	cmdMap := make(map[string]string)

	log.Printf("Registering handler for /runlog page...\n")
	http.HandleFunc("/runlog", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "text/html")
		io.WriteString(w, `
				<html>
				<head>
				<meta http-equiv="refresh" content="10">
				</head>
				<body>
				`)

		rl, _ := ioutil.ReadFile("run.log")

		// Split log into header and the rest, with endpoints
		// at top and events below, so as log gets longer user
		// can still see important bits.
		lines := strings.Split(string(rl), "--GOFISH READY--")
		tailLines := strings.Split(lines[1], "\n")
		tailCount := len(tailLines)

		io.WriteString(w, "<pre style='background-color: skyblue;'>")
		io.WriteString(w, lines[0]+"...")
		io.WriteString(w, "</pre>")

		io.WriteString(w, "<pre>")
		if runLogTailLines == 0 || tailCount < runLogTailLines {
			io.WriteString(w, strings.Join(tailLines, "\n"))
		} else {
			io.WriteString(w, strings.Join(tailLines[tailCount-runLogTailLines:], "\n"))
		}
		io.WriteString(w, "</pre>")
		//

		io.WriteString(w, `
				</body>
				</html>
				`)
	})

	jobHomeDir = "workdir"
	// Each non-switch argument is taken to be an endpoint (job) descriptor
	// Syntax of an endpoint:
	//  endpoint:jobDir:EVAR1=val1,EVAR2=val2[,...,EVAR<n>=val<n>]:cmd
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

		// Launch webhook listeners for each defined endpoint
		// Note presently only 'blind' hookStd is supported
		// (ie., if webhook request contains POST JSON data,
		// it isn't read).
		if len(tag) > 0 {
			log.Printf("<a href='%s/%s'>[&gt;]</a>Registering handler for %s/%s [action %s]...\n",
				hookStd, tag, hookStd, tag, cmd)
			launchJobListener(mainCtx, tag, jobEnv, cmdMap)
		}
	}
	log.Printf("--GOFISH READY--\n")

	// A single endpoint handles the 'live' job output
	http.HandleFunc("/"+jobHomeDir+"/", consoleHandler)
	// Similarly, a single endpoint handles static full job output
	http.HandleFunc("/"+jobHomeDir+"/fullconsole/", fullConsoleHandler)

	err := http.ListenAndServe(addrPort, nil)
	if err != nil {
		log.Fatal(err)
	}
}
