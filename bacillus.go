// Package bacillus is a Webhook listener that dispatches
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
	"math/rand"
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
	addrPort     string
	hookStd      string
	apiKey       string
	attachStdout bool
	//statUseUnicode  bool
	indStyle        string
	instCounter     uint32
	jobHomeDir      string
	artifactBaseDir string
	runLogTailLines int

	//checkSeq string
	//errSeq   string
	//playSeq  string

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

func fullRunlogHandler(w http.ResponseWriter, r *http.Request) {
	runLog, e := ioutil.ReadFile("run.log")
	if e != nil {
		w.Write([]byte(fmt.Sprintf("%s", e)))
		return
	}
	w.Header().Set("Content-type", "text/html")
	io.WriteString(w, "<html><head></head><body><pre>\n")
	w.Write(runLog)
	io.WriteString(w, "</pre></body</html>\n")
}

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

	var stat rune
	var code int //TODO: use to determine red failure marker?
	n, e := fmt.Sscanf(consStat, "[%c %03d]", &stat, &code)
	_ = n
	if l > 0 {
		tail = lines[len(lines)-tailL:]
		_ = fullConsLink
		consoleLog = []byte("<a href=\"" + fullConsLink + "\">full log</a>\n" + strings.Join(tail, "\n"))
	} else {
		tail = lines[2:]
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
	instColours := []string{
		"floralwhite",
		"burlywood",
		"cadetblue",
		"chocolate",
		"coral",
		"cornflowerblue",
		"cornsilk",
		"darkcyan",
		"darkgoldenrod",
		"darkgrey",
		"darkkhaki",
		"darkorange",
		"darksalmon",
		"darkseagreen",
		"darkturquoise",
		"gainsboro",
		"gold",
		"goldenrod"}

	http.HandleFunc(fmt.Sprintf("/%s/%s", hookStd, tag),
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(fmt.Sprintf("Triggered %s", tag)))
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
				defer cmdCancelFunc()
				var cmdPrefix string
				if cmdStrList[0][0] == '/' {
					cmdPrefix = ""
				} else {
					cmdPrefix = "../"
				}

				c := exec.CommandContext(cmdCancelCtx, cmdPrefix+cmdStrList[0], cmdArgs...)

				//var terr error
				//var workDir string
				var instColourIdx uint32
				if indStyle == "colour" || indStyle == "both" {
					instColourIdx = rand.Uint32() % uint32(len(instColours))
					instCounter += 1
				} else {
					instColourIdx = 0
				}
				instColour := instColours[instColourIdx]

				dirTmp, _ := filepath.Abs(jobHomeDir)
				workDir, terr := ioutil.TempDir(dirTmp, "bacillus_")
				c.Dir = workDir
				jobID := strings.Split(workDir, "_")[1]

				var indent int64
				var indentStr string
				if indStyle == "indent" || indStyle == "both" {
					indent, _ = strconv.ParseInt(jobID, 10, 64)
					indentStr = strings.Repeat("-", int(indent%8)+1)
				}

				if terr != nil {
					log.Printf("[ERROR creating workdir (%s) for event %s trigger.]\n", terr, tag)
				} else {

					// TODO: Spawn http.HandleFunc() or ? here to serve out
					// /artifacts/ virtual URIs (See go http.FileSystem?)
					// NOTE below won't work -- here the lifetime only is as long
					// as the job, we want a handler that can look at the
					// artifacts/ subdir(s) anytime after a job runs.
					//http.Handle(fmt.Sprintf("/artifacts/%s", jobID),
					//		http.FileServer(http.Dir(fmt.Sprintf("/%s/bacillus_%s/%s/", jobHomeDir, jobID, "artifacts"))))

					var workerOutputPath string
					var workerOutputFile *os.File
					consoleFName := "console.out"
					workerOutputPath = workDir + "/" + consoleFName
					workerOutputRelPath := fmt.Sprintf("%s/bacillus_%s/%s", jobHomeDir, jobID, consoleFName)
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
					c.Env = append(c.Env, fmt.Sprintf("BACILLUS_JOBID=%s", jobID))
					c.Env = append(c.Env, fmt.Sprintf("BACILLUS_JOBTAG=%s", tag))
					c.Env = append(c.Env, fmt.Sprintf("BACILLUS_WORKDIR=%s", workDir))
					c.Env = append(c.Env, fmt.Sprintf("BACILLUS_ARTFDIR=%s", fmt.Sprintf("%s/../../artifacts/%s_%s", workDir, tag, jobID)))
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
						log.Printf("<span style='background-color:%s'>%s<a href='%s'>[o]</a>[event %s{%s}<a href='/cancel/%s'>[X]</a> triggered. (workDir %s)]</span>\n", instColour, indentStr,
							workerOutputRelPath,
							tag,
							jobID,
							jobID,
							workDir)
						//log.Printf("%s[console log:<a href=\"%s\">%s</a>]\n", indentStr,
						//	workerOutputRelPath, workerOutputRelPath)

						// Spawn handler for /cancel/<jobID>
						http.HandleFunc(fmt.Sprintf("/cancel/%s", jobID),
							func(w http.ResponseWriter, r *http.Request) {
								if jobCancellers[jobID] != nil {
									jobCancellers[jobID]()
									//delete(jobCancellers, jobID)
									w.Write([]byte(fmt.Sprintf("Cancelled %s", jobID)))
								} else {
									w.Write([]byte(fmt.Sprintf("Job %s already done or not found.", jobID)))
								}
							})
					}
					werr := c.Wait()
					//jobCancellers[jobID]()
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
							fmt.Fprintf(workerOutputFile, "[f %03d]", int8(exitStatus))
							//log.Print(c.Stderr /*stdErrBuffer*/)
							//log.Printf("%s[Exit Status: %d]\n", indentStr, int32(exitStatus)) //#
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
						log.Printf("<span style='background-color:%s'>%s<a href='/artifacts/%s_%s'>[&check;]</a>[event %s{<a href='%s'>%s</a>} completed with status 0]</span>\n", instColour, indentStr,
							tag, jobID,
							tag, workerOutputRelPath, jobID)
					} else {
						log.Printf("<span style='background-color:%s'>%s<span style='background-color:red'><a href='/artifacts/%s_%s'>[!]</a></span>[event %s{<a href='%s'>%s</a>} completed with error %s]</span>\n", instColour, indentStr,
							tag, jobID,
							tag, workerOutputRelPath, jobID, werr)
					}
					if strings.Contains(strings.Join(c.Env, " "),
						"BACILLUS_REMOVE_WORKDIR") {
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
	flag.StringVar(&indStyle, "i", "both", "job entry indicator style [none|indent|colour|both]")
	flag.IntVar(&runLogTailLines, "rl", 32, "Scroll length of runlog (set to 0 for no limit)")
	flag.BoolVar(&attachStdout, "s", false, "set to true to see worker stdout/err if running in terminal")
	//flag.BoolVar(&statUseUnicode, "S", true, "set to false to use plain ASCII (ISO-8859-1) in /runlog")
	flag.Parse()

	mainCtx := context.Background()
	jobCancellers = make(map[string]func())

	logfile, _ := os.Create("run.log")
	log.SetOutput(logfile)

	log.Printf("[bacillus %s startup]\n", appVer)
	log.Printf("[listening on %s, type %s]\n", addrPort, hookStd)

	cmdMap := make(map[string]string)

	log.Printf("Registering handler for /runlog page.\n")
	http.HandleFunc("/runlog", func(w http.ResponseWriter, r *http.Request) {
		//if statUseUnicode {
		//	checkSeq = "o"
		//	playSeq = string([]byte{'&','#'})
		//	errSeq = "X"
		//} else {
		//	checkSeq = "o"
		//	playSeq = ">"
		//	errSeq = "X"
		//}

		w.Header().Set("Content-type", "text/html")
		io.WriteString(w, `
				<html>
				<head>
				<meta http-equiv="refresh" content="5">
				</head>
				<body>
				`)

		rl, _ := ioutil.ReadFile("run.log")

		// Split log into header and the rest, with endpoints
		// at top and events below, so as log gets longer user
		// can still see important bits.
		lines := strings.Split(string(rl), "--BACILLUS READY--")
		tailLines := strings.Split(lines[1], "\n")
		tailCount := len(tailLines)

		io.WriteString(w, "<pre style='background-color: skyblue;'>")
		io.WriteString(w, lines[0]+"<a href='/fullrunlog'>...</a>")
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
			log.Printf("<a href='%s/%s'>[&#9654;]</a>Registering handler for %s/%s [action %s].\n",
				hookStd, tag,
				hookStd, tag, cmd)
			launchJobListener(mainCtx, tag, jobEnv, cmdMap)
		}
	}
	log.Printf("--BACILLUS READY--\n")

	// Make a filesystem available for dir/file storage & retrieval by
	// jobs and devs. Jobs are responsible for its proper use.
	artifactBaseDir, aerr := filepath.Abs("artifacts")
	if aerr == nil {
		http.Handle("/artifacts/",
			http.StripPrefix("/artifacts/", http.FileServer(http.Dir(artifactBaseDir))))
	}

	// Live runlog is just the tail of full runlog
	http.HandleFunc("/fullrunlog/", fullRunlogHandler)

	// A single endpoint handles the 'live' job output
	http.HandleFunc("/"+jobHomeDir+"/", consoleHandler)
	// Similarly, a single endpoint handles static full job output
	http.HandleFunc("/"+jobHomeDir+"/fullconsole/", fullConsoleHandler)

	// And finally, the root fallback to give help on defined endpoints.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "text/html")
		io.WriteString(w, `
							<html>
							<head>
							</head>
							<body>
							`)
		io.WriteString(w, `
  <pre>
  <a href='/runlog'>/runlog</a>: main log/activity view
  <a href='/artifacts'>/artifacts</a>: where jobs (should) leave their stuff
  .. that's about it.
  </pre>
							`)
		io.WriteString(w, `
							</body>
							</html>
							`)
	})

	//go func() {
	//	log.Fatal(http.ListenAndServe(":9991", http.FileServer(http.Dir(jobHomeDir))))
	//}()

	log.Fatal(http.ListenAndServe(addrPort, nil))
}
