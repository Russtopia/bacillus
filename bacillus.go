// Package bacillus is a Webhook listener that dispatches
// arbitrary commands on receipt of webhook events.
// Supported webhook event formats: gogs, (.. future)
//
// Qui verifiers ratum efficiat?
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

func getGoBackHeaderJS(ms string) string {
	return fmt.Sprintf(`
<script>
  // Go back after a short delay
  setInterval(function(){ window.location.href = document.referrer; }, %s);
</script>
`, ms)
}

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

func getRunLogCSS() string {
	return `
		<style>
		a:link { text-decoration:none; }
		a:hover { text-decoration:underline; }
		a:active { text-decoration:underline; }
		</style>
		`
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
	runLog, e := ioutil.ReadFile(fmt.Sprintf("run%s.log", strings.Split(addrPort, ":")[1]))
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

func launchJobListener(mainCtx context.Context, jobTag, jobOpts string, jobEnv []string, cmdMap map[string]string) {
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

	http.HandleFunc(fmt.Sprintf("/%s/%s", hookStd, jobTag),
		func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `
					<html>
					<head>`+
				getGoBackHeaderJS("3000")+`
					</head>
                    <body>
					`)
			io.WriteString(w, fmt.Sprintf("<pre>Triggered %s</pre>\n", jobTag))
			io.WriteString(w, `
					</body>
					</html>`)

			go func() {
				// Some wrinkles in the exec.Command API: If there are no args,
				// one must completely omit the args ... to avoid strange errors
				// with some commands that see a blank "" arg and complain.
				cmd := strings.Split(cmdMap[jobTag], " ")[0]
				cmdStrList := strings.Split(cmdMap[jobTag], " ")[1:]
				//fmt.Printf("%s %v\n", cmd, cmdStrList)
				cmdCancelCtx, cmdCancelFunc := context.WithCancel(mainCtx)
				defer cmdCancelFunc()
				//var args string
				var c *exec.Cmd
				if len(cmdStrList) > 0 {
					c = exec.CommandContext(cmdCancelCtx, cmd, strings.Join(cmdStrList, " "))
				} else {
					c = exec.CommandContext(cmdCancelCtx, cmd)
				}

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
				workDir, terr := ioutil.TempDir(dirTmp, fmt.Sprintf("bacillus_%s_", jobOpts))
				c.Dir = workDir
				jobID := strings.Split(workDir, "_")[2]

				var indent int64
				var indentStr string
				if indStyle == "indent" || indStyle == "both" {
					indent, _ = strconv.ParseInt(jobID, 10, 64)
					indentStr = strings.Repeat("-", int(indent%8)+4)
				}

				if terr != nil {
					log.Printf("[ERROR creating workdir (%s) for job %s trigger.]\n", terr, jobTag)
				} else {
					var workerOutputPath string
					var workerOutputFile *os.File
					consoleFName := "console.out"
					workerOutputPath = workDir + "/" + consoleFName
					workerOutputRelPath := fmt.Sprintf("%s/bacillus_%s_%s/%s", jobHomeDir, jobOpts, jobID, consoleFName)
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
					c.Env = append(c.Env, fmt.Sprintf("BACILLUS_JOBTAG=%s", jobTag))
					c.Env = append(c.Env, fmt.Sprintf("BACILLUS_WORKDIR=%s", workDir))
					c.Env = append(c.Env, fmt.Sprintf("BACILLUS_ARTFDIR=%s", fmt.Sprintf("%s/../../artifacts/bacillus_%s_%s", workDir, jobOpts, jobID)))
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
						log.Printf("%s[ERROR on job %s trigger.]\n", indentStr,
							jobTag)
					} else {
						jobCancellers[jobID] = cmdCancelFunc
						w.Write([]byte("OK"))
						log.Printf("<!--JOBID:%s:JOBID--><span style='background-color:%s'><a style='display:inline;' href='%s' title='Running'>[&acd;]</a>%s[job %s{%s}<a style='display:inline;' href='/cancel/%s' title='Cancel'>[&cross;]</a> triggered.]</span>\n",
							jobID, instColour,
							workerOutputRelPath,
							indentStr,
							jobTag, jobID,
							jobID)
						//log.Printf("%s[console log:<a href=\"%s\">%s</a>]\n", indentStr,
						//	workerOutputRelPath, workerOutputRelPath)

						// Spawn handler for /cancel/<jobID>
						http.HandleFunc(fmt.Sprintf("/cancel/%s", jobID),
							func(w http.ResponseWriter, r *http.Request) {
								io.WriteString(w, `
					<html>
					<head>`+
									getGoBackHeaderJS("3000")+`
					</head>
					<body>
					`)
								if jobCancellers[jobID] != nil {
									jobCancellers[jobID]()
									io.WriteString(w, fmt.Sprintf("<pre>Cancelled %s</pre>\n", jobID))
								} else {
									io.WriteString(w, fmt.Sprintf("<pre>Job %s already done or not found.</pre>\n", jobID))
								}
								io.WriteString(w, `
					</body>
					</html>`)
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
						log.Printf("<!--JOBID:%s:JOBID--><span style='background-color:%s'><a href='%s' title='Done'>[&check;]</a>%s[job %s{%s}<a href='/artifacts/bacillus_%s_%s' title='Artifacts'>[&ccupssm;]</a> completed with status 0]</span><!--COMPLETION-->\n",
							jobID, instColour,
							workerOutputRelPath,
							indentStr,
							jobTag, jobID,
							jobOpts, jobID)
					} else {
						log.Printf("<!--JOBID:%s:JOBID--><span style='background-color:%s'><span style='background-color:red'><a href='%s' title='Done With Errors'>[!]</a></span>%s[job %s{%s}<a href='/artifacts/bacillus_%s_%s' title='Partial Artifacts'>[&ccups;]</a> completed with error %s]</span><!--COMPLETION-->\n",
							jobID, instColour,
							workerOutputRelPath,
							indentStr,
							jobTag, jobID,
							jobOpts, jobID,
							werr)
					}
					if strings.Contains(strings.Join(c.Env, " "),
						"BACILLUS_REMOVE_WORKDIR") {
						_ = os.RemoveAll(workDir)
					}
				}
			}()
		})
}

//FIXME: There is definitely a less copy-intensive way to do this.
func patchCompletedJobsInLog(orig []string, horizon int) (fixed []string) {
	// The data being patched is a 'live' view, limited to a tail
	// portion of the actual run.log. In light of this we only need
	// to reconcile finished jobs back a short distance, larger than
	// the displayed tail length, in lines.
	// Jobs should take longer than a few seconds, which is the
	// refresh interval of the /runlog endpoint; so there *should*
	// only be a very small number of entries that have completed
	// since the last scan and still visible on the 'live' web view.
	// We'll only look for those few, so if there was a spamming run of
	// short-lived jobs, we might not mark all of them as completed.
	// Meh. Not worth an O(n^2) operation.
	fixed = orig

	// As described above, prevent excessive processing for live web view
	if horizon > 255 {
		horizon = 255
	}

	l := len(fixed) - 1
	if l > 1 {
		if l > horizon {
			horizon = l - horizon
		} else {
			horizon = 0
		}

		for idx := l; idx > horizon; idx-- {
			//fmt.Println("idx:", idx, "l:", l, "horizon:", horizon)
			if strings.Count(fixed[idx], "<!--COMPLETION-->") != 0 {
				//fmt.Println("found COMPLETION")
				// Found a completed job. Seek a few entries back
				// to mark the job launch stmt, hiding the in-progress
				// and cancel links within.
				var jidStart, jidEnd int
				var jobID string
				jidStart = strings.Index(fixed[idx], "<!--JOBID:")
				if jidStart != -1 {
					jidStart += len("<!--JOBID:")
					jidEnd = strings.Index(fixed[idx], ":JOBID-->")
				}
				if jidStart != -1 && jidEnd != -1 {
					jobID = fixed[idx][jidStart:jidEnd]
					jobTag := "<!--JOBID:" + jobID + ":JOBID-->"
					//fmt.Printf("found %s\n", jobTag)
					for seekIdx := idx - 1; seekIdx >= 0 && seekIdx > horizon; seekIdx-- {
						// NOTE we're modifying the 'live' view of
						// the logfile, not the direct data on disk, so
						// no need to replace byte-for-byte.
						// (If this func is optimized to be zero-copy
						//  however, it might need to be.)
						if strings.Contains(fixed[seekIdx], jobTag) {
							//fmt.Println("found jobTag, patching")
							fixed[seekIdx] = strings.Replace(fixed[seekIdx],
								"display:inline", "display:none", -1)
							if indStyle == "both" || indStyle == "indent" {
								fixed[seekIdx] = strings.Replace(fixed[seekIdx],
									"---", "------", 1)
							} else if indStyle == "colour" {
								fixed[seekIdx] = strings.Replace(fixed[seekIdx],
									"[job", "   [job", 1)
							}
						}
					}
				}
			}
		}
	}
	//fmt.Println(fixed)
	return fixed
}

func main() {
	flag.StringVar(&addrPort, "a", ":9990", "[addr]:port on which to listen")
	flag.StringVar(&hookStd, "h", "blind", "hook type")
	flag.StringVar(&apiKey, "k", defKey, "API key")
	flag.StringVar(&indStyle, "i", "both", "job entry indicator style [none|indent|colour|both]")
	flag.IntVar(&runLogTailLines, "rl", 30, "Scroll length of runlog (set to 0 for no limit)")
	flag.BoolVar(&attachStdout, "s", false, "set to true to see worker stdout/err if running in terminal")
	//flag.BoolVar(&statUseUnicode, "S", true, "set to false to use plain ASCII (ISO-8859-1) in /runlog")
	flag.Parse()

	mainCtx := context.Background()
	jobCancellers = make(map[string]func())

	logfile, _ := os.Create(fmt.Sprintf("run%s.log", strings.Split(addrPort, ":")[1]))

	log.SetOutput(logfile)
	log.Printf("[bacillus %s startup] <a href='/'>usage</a>\n", appVer)
	log.Printf("[listening on %s, type %s]\n", addrPort, hookStd)

	cmdMap := make(map[string]string)

	//log.Printf("Registering handler for /runlog page.\n")
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
				<meta http-equiv="refresh" content="5">`+
			getRunLogCSS()+`
				</head>
				<body>
				`)

		rl, _ := ioutil.ReadFile(fmt.Sprintf("run%s.log", strings.Split(addrPort, ":")[1]))

		// Split log into header and the rest, with endpoints
		// at top and events below, so as log gets longer user
		// can still see important bits.
		lines := strings.Split(string(rl), "--BACILLUS READY--")
		tailLines := strings.Split(lines[1], "\n")
		tailCount := len(tailLines)

		//TODO: scan backwards in log for completion msgs, match with
		// preceding launch msgs to un-mark the in-progress and cancel icons there
		tailLines = patchCompletedJobsInLog(tailLines, runLogTailLines)

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
	//  endpoint:jobOpts:EVAR1=val1,EVAR2=val2[,...,EVAR<n>=val<n>]:cmd
	for _, e := range flag.Args() {

		fields := strings.Split(e, ":")
		var tag string
		var jobOpts string
		var jobEnv []string
		var cmd string
		if fields[0] != e {
			tag = fields[0]
			if len(fields) > 1 && len(fields) != 4 {
				errStr := fmt.Sprintf("\n  [%s]\n"+
					"  All endpoint specs must have exactly 4 fields:\n"+
					"    endpoint:jobOpts:envVars:cmd\n"+
					"  (jobOpts and envVars may be empty.)\n",
					fields[0])
				fmt.Print(errStr)
				log.Fatal(errStr)
			}

			jobOpts = fields[1]
			_ = jobOpts
			jobEnv = strings.Split(fields[2], ",")
			cmd = fields[3]

			cmdMap[tag] = cmd

			// Launch webhook listeners for each defined endpoint
			// Note presently only 'blind' hookStd is supported
			// (ie., if webhook request contains POST JSON data,
			// it isn't read).
			if len(tag) > 0 {
				//log.Printf("<a href='%s/%s'>[&#9654;]</a>%s/%s [action %s]\n",
				log.Printf("<a href='%s/%s' title='Play Job'>[&rtrif;]</a>%s/%s [action %s]\n",
					hookStd, tag,
					hookStd, tag, cmd)

				launchJobListener(mainCtx, tag, jobOpts, jobEnv, cmdMap)
			}
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

	http.Handle("/images/",
		http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))

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
							<body style='background-image: linear-gradient(to bottom, rgba(0,0,0,0.1) 0%,rgba(0,0,0,0.8) 100%), url("/images/bacillus.jpg"); background-size: cover;'>
							`)
		io.WriteString(w, `
  <pre>
  <a href='/runlog'>/runlog</a>: main log/activity view
  <a href='/artifacts'>/artifacts</a>: where jobs (should) leave their stuff
  
  LEGEND
  [&rtrif;] Start a job manually
  [&cross;] Cancel a running job
  [&ccupssm;] View completed job artifacts
  [&ccups;] View partial artifacts for a failed job
  [&acd;] Job is running - click to view in-progress output
  [&check;] Job has completed with OK(0) status - click to view output
  <span style='background-color:red'>[!]</span> Job has exited with nonzero status - click to view output

  .. that's about it.
     Happy Build Automating, DevOps-ing, or whatever it's called these days...
	 </pre>
	 <span style='font-size: 8px; position: fixed; bottom: 0; left: 10;'><pre>Qui verifiers ratum efficiat? Non I.</pre></span>
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
