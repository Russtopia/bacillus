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
	"syscall"
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

	var refreshStr string
	var spinnerCode string
	var codeColor string
	var statWord string
	if code != 0 {
			codeColor = "finErrMarker"
			statWord = "ERR"
	} else {
			codeColor = "finOKMarker"
			statWord = "Done"
	}

	if stat == 'r' {
		refreshStr = `<meta http-equiv="refresh" content="5">`
		spinnerCode = `appendSpinner = function() {
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
`
	} else {
		refreshStr = ``
		spinnerCode = `appendSpinner = function() {
					var el = document.createElement('div');
					el.setAttribute('id', '` + codeColor + `');
					el.innerHTML = '`+statWord+`';
					document.body.appendChild(el);
					}
					`
	}

	w.Header().Set("Content-type", "text/html")
	io.WriteString(w, `
				<html>
				<head>
				`+refreshStr+`
				<style>
				  #spinner {
				    position: fixed;
					right: 1em; bottom: 1em;
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
						  right: 1em; bottom: 1em;
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
						  right: 1em; bottom: 1em;
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
					//display: none;
				  //}
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
					setTimeout (function () {
					  bodyOrHtml().scrollTop = bodyOrHtml().scrollHeight;
	  				}, 5); // hack: delay due to most browsers' auto-scroll reset on page reload
				  }
`+spinnerCode+`

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
	//} else {
	//	w.WriteHeader(http.StatusNotFound)
	//	fmt.Fprint(w, "404 - Page Not Found")
	//}
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
		w.Header().Set("Content-type", "text/html")
		io.WriteString(w, `
				<html>
				<head>
				<meta http-equiv="refresh" content="10">
				</head>
				<body>
				`)
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
							w.Write([]byte("OK"))
							log.Printf("%s[event %s{%s} triggered. (workDir %s)]\n", indentStr,
								tag, jobID, workDir)
							log.Printf("%s[console log:<a href=\"%s\">%s</a>]\n", indentStr,
								workerOutputRelPath, workerOutputRelPath)
						}
						werr := c.Wait()

						//						if workerOutputFile != nil {
						//								offs, serr := workerOutputFile.Seek(0, 0)
						//								fmt.Println(offs)
						//							if serr != nil {
						//								fmt.Printf(fmt.Sprintf("%s", serr))
						//							}
						//						}

						if werr, ok := werr.(*exec.ExitError); ok {
							// The program has exited with an exit code != 0

							// This works on both Unix and Windows. Although package
							// syscall is generally platform dependent, WaitStatus is
							// defined for both Unix and Windows and in both cases has
							// an ExitStatus() method with the same signature.
							var exitStatus uint32
							if status, ok := werr.Sys().(syscall.WaitStatus); ok {
								exitStatus = uint32(status.ExitStatus())
								workerOutputFile, _ = os.OpenFile(workerOutputPath, os.O_RDWR, 0777)
								fmt.Fprintf(workerOutputFile, "[f %03d]", exitStatus)
								//log.Print(c.Stderr /*stdErrBuffer*/)
								log.Printf("Exit Status: %d\n", exitStatus) //#
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
	http.HandleFunc("/"+jobHomeDir+"/fullconsole/", fullConsoleHandler)

	err := http.ListenAndServe(addrPort, nil)
	if err != nil {
		log.Fatal(err)
	}
}
