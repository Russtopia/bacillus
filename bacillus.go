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
	"sort"

	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	appVer         string = "v0.1"
	httpAuthUser          = "bacuser"
	httpAuthPasswd        = "gramnegative" //b64:"YmFjdXNlcjpncmFtbmVnYXRpdmU="
)

var (
	server             *http.Server
	addrPort           string
	basicAuth          bool   // basic http auth
	strUser            string // API user
	strPasswd          string // API passwd
	apiKey             string
	attachStdout       bool
	shutdownModeActive bool
	killSwitch         chan bool
	//statUseUnicode  bool
	indStyle    string
	instCounter uint32
	//runningJobCount uint
	cmdMap          map[string]string
	runningJobs     RunningJobList //map[string]string
	jobHomeDir      string
	artifactBaseDir string
	runLogTailLines int

	//checkSeq string
	//errSeq   string
	//playSeq  string

	jobCancellers map[string]func()

	instColours = []string{
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
)

type RunningJobList map[string]string

// There is a smattering of HTML and JS in this project, programmatically
// generated.
// No, I did not use templates.
// Yes, I may rewrite in the future to do so, but don't hold your breath.
// I didn't design this thing up-front, I wrote it to scratch an itch.
// That's what 'agile design' gets you :p

func xhrlinkCSSFrag() string {
	return `<style>
		span.xhrlink:hover {
				text-decoration: underline;
				background-color: aliceblue;
				cursor: pointer;
		}
		span.xhrlink:active {
				background-color: lightgreen;
		}
		</style>
		`
}

// Emit JS function suitable for calling from an html element
// Typically used for an onclick event to fire off an async GET req.
func xmlHTTPRequester(jsFuncName string, uri string, respHandlerJS string) string {
	return `
<script>
	function ` + jsFuncName + `() {
		// IDGAF about IE 5/6, nor should you
		var xhttp = new XMLHttpRequest();
		xhttp.onreadystatechange = function() {
			if( this.readyState == 4 && this.status == 200 ) {
					// whatevs, maybe give feedback to user
					` + respHandlerJS + `
			}
		};
		xhttp.open('GET', '` + uri + `', true);
		xhttp.send();
	}
</script>`
}

func xhrRunningJobsCountHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, fmt.Sprintf("%d", len(runningJobs)))
}

func xhrLiveRunLogHandler(w http.ResponseWriter, r *http.Request) {
	tl := 4
	v, ok := r.URL.Query()["tl"]
	if ok {
		fmt.Sscanf(v[0], "%d", &tl)
	}
	io.WriteString(w, liveRunLogHTML(tl))
}

func favIconHTML() string {
	return `<link rel="icon" type="image/jpg" href="/images/logo.jpg"/>`
}

func logoShortHdrHTML() string {
	return `<img style='float:left;' width='16px' src='/images/logo.jpg'/><pre><a href='/'>bacill&mu;s ` + appVer + `</a></pre>`
}

func logoHdrHTML() string {
	return `<img style='float:left;' width='16px' src='/images/logo.jpg'/><pre><a href='/'>bacill&mu;s ` + appVer + ` <a href='https://gogs.blitter.com/Russtopia/bacillus/src/master/README.md'>(What's this?)</a></pre>`
}

func bodyBgndHTMLAttribs() string {
	if shutdownModeActive {
		return ` style='background: linear-gradient(to bottom, rgba(0,0,0,0.1) 0%,rgba(0,0,0,0.8) 100%); background-image: url("/images/bacillus-shutdown.jpg"); background-size: cover;'`
	}
	return ` style='background: linear-gradient(to bottom, rgba(0,0,0,0.1) 0%,rgba(0,0,0,0.8) 100%); background-image: url("/images/bacillus.jpg"); background-size: cover;'`
}

// goBackJS() returns a JS fragment to make a page go back after a
// short delay.
func goBackJS(ms string) string {
	return fmt.Sprintf(`
<script>
  // Go back after a short delay
  setInterval(function(){ window.location.href = document.referrer; }, %s);
</script>
`, ms)
}

func refreshJS(stat rune, intervalSecs string) string {
	if stat == 'r' {
		return `<meta http-equiv="refresh" content="` + intervalSecs + `">`
	} else {
		return ``
	}
}

func consActiveSpinnerCSS() string {
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

func consActiveSpinnerJS(stat rune, codeColor, statWord string) string {
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

func compatJS() string {
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

// Get HTML for 'live' runlog with a specified # of tail lines
// Mean to be inserted within a serve-out complete HTML page
// (for just an HTML fragment to be inserted by client-side
//  see xurLiveRunLogHandler())
func liveRunLogHTML(tl int) (ret string) {
	rl, _ := ioutil.ReadFile(fmt.Sprintf("run%s.log", strings.Split(addrPort, ":")[1]))

	// Split log into header and the rest, with endpoints
	// at top and events below, so as log gets longer user
	// can still see important bits.
	lines := strings.Split(string(rl), "--BACILLUS READY--")
	tailLines := strings.Split(lines[1], "\n")
	tailCount := len(tailLines)

	// Scan backwards in log for completion msgs, match with
	// preceding launch msgs to un-mark the in-progress and cancel icons there
	// (only 'live' view)
	tailLines = patchCompletedJobsInLog(tailLines, tl)

	if tl == 0 || tailCount < tl {
		ret += strings.Join(tailLines, "\n")
	} else {
		ret += strings.Join(tailLines[tailCount-tl:], "\n")
	}

	return
}

func manualJobTriggersJS() (ret string) {
	// Put in the click JS functions first
	keys := make([]string, len(cmdMap))
	for k := range cmdMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if len(cmdMap[k]) > 0 {
			fn := strings.Replace(k, "-", "", -1)
			ret += xmlHTTPRequester(fn, k, "")
			ret += `<script>
			setInterval( xhrLiveRunLogUpdate, 1000 );
			setInterval( xhrRunningJobsCount, 1000 );
			</script>`
		}
	}
	return
}

func manualJobTriggersHTML(fullLogLink bool) (ret string) {
	ret = "<pre style='background-color: skyblue;'>"
	keys := make([]string, len(cmdMap))
	for k := range cmdMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if len(cmdMap[k]) > 0 {
			//			ret += fmt.Sprintf("<a href='%s' title='Play Job'>[&rtrif;]</a>%s [action %s]\n",
			//				k, k, cmdMap[k])
			fn := strings.Replace(k, "-", "", -1)
			//ret += fmt.Sprintf("<a href='' onclick='%s()' title='Play Job'>[&rtrif;]</a>%s [action %s]\n",
			ret += fmt.Sprintf("<span class='xhrlink' onclick='%s()' title='Play Job'>[&rtrif;] %s [action %s]</span>\n",
				/*k,*/ fn, k, cmdMap[k])
		}
	}
	if fullLogLink {
		ret += "<a href='/fullrunlog'>... click for full runlog ...</a>"
	}
	ret += "</pre>"
	return
}

func httpAuthSession(w http.ResponseWriter, r *http.Request) (auth bool) {
	w.Header().Set("Cache-Control", "no-cache")

	if !basicAuth {
		return true
	}

	u, p, ok := r.BasicAuth()
	if ok && u == strUser && p == strPasswd {
		return true
	} else {
		w.Header().Set("WWW-Authenticate", `Basic realm="Bacillus"`)
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, "Not logged in.")
	}
	return
}

// TODO: types for matching JSON events of
// supported webhooks: gogs.io, github, gitlab, ... ?
// For now, the 'blind' endpoint is the only one supported,
// meaning the request can't communicate any extra data to the
// job invocation in a GET or POST request.

func runLogHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html")
	if !httpAuthSession(w, r) {
		return
	}

	io.WriteString(w, `
<html>
<head>`+
		favIconHTML()+
		xhrlinkCSSFrag()+
		xmlHTTPRequester("xhrLiveRunLogUpdate", fmt.Sprintf("/api/lru?tl=%d", runLogTailLines), `document.getElementById('liveRunLog').innerHTML = xhttp.response;`)+
		logoShortHdrHTML()+`
</head>
<body `+bodyBgndHTMLAttribs()+`>`)
	io.WriteString(w, manualJobTriggersHTML(true)+
		`<pre id='liveRunLog'>`+liveRunLogHTML(runLogTailLines)+`</pre>`)

	io.WriteString(w, manualJobTriggersJS())
	io.WriteString(w, `
</body>
</html>
    `)
}

func fullRunlogHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html")
	if !httpAuthSession(w, r) {
		return
	}

	runLog, e := ioutil.ReadFile(fmt.Sprintf("run%s.log", strings.Split(addrPort, ":")[1]))
	if e != nil {
		w.Write([]byte(fmt.Sprintf("%s", e)))
		return
	}
	io.WriteString(w, `
<html>
<head>`+
		favIconHTML()+
		logoShortHdrHTML()+`
</head>
<body>`)

	io.WriteString(w, `
<pre>
`+string(runLog)+`
</pre>
</body>
</html>`)
}

func fullConsoleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/plain")
	if !httpAuthSession(w, r) {
		return
	}

	consoleLog, e := ioutil.ReadFile(strings.Replace(fmt.Sprintf("%s", r.URL)[1:], "/fullconsole", "", 1))
	if e != nil {
		w.Write([]byte(fmt.Sprintf("%s", e)))
		return
	}
	w.Write(consoleLog)
}

func consoleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html")
	if !httpAuthSession(w, r) {
		return
	}

	// Read file from URL, removing leading / as workdir is rel to us
	consoleLog, e := ioutil.ReadFile(fmt.Sprintf("%s", r.URL)[1:])
	if e != nil {
		w.Write([]byte(fmt.Sprintf("%s", e)))
		return
	}

	lines := strings.Split(string(consoleLog), "\n")
	// Prevent log output from creating huge web pages.
	tailL := 34
	l := len(lines) - tailL
	if l < 0 {
		l = 0
	}
	consStat := lines[0]
	fullConsLink := lines[1]
	//jobTag := lines[2]

	var tail []string

	var stat rune
	var code int
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

	io.WriteString(w, `
<html>
<head>
`+
		favIconHTML()+
		refreshJS(stat, "5")+
		compatJS()+
		consActiveSpinnerCSS()+
		consActiveSpinnerJS(stat, codeColor, statWord)+
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

type jobCtx struct {
	w       http.ResponseWriter
	mainCtx context.Context
	jobTag  string
	jobOpts string
	jobEnv  []string
}

func execJob(j jobCtx) {
	// Some wrinkles in the exec.Command API: If there are no args,
	// one must completely omit the args ... to avoid strange errors
	// with some commands that see a blank "" arg and complain.
	cmd := strings.Split(cmdMap[j.jobTag], " ")[0]
	cmdStrList := strings.Split(cmdMap[j.jobTag], " ")[1:]
	//fmt.Printf("%s %v\n", cmd, cmdStrList)
	cmdCancelCtx, cmdCancelFunc := context.WithCancel(j.mainCtx)
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
	workDir, terr := ioutil.TempDir(dirTmp, fmt.Sprintf("bacillus_%s_%s_", j.jobOpts, j.jobTag))
	c.Dir = workDir
	jobID := strings.Split(workDir, "_")[3]
	//fmt.Println("jobID:", jobID)
	var indent int64
	var indentStr string
	if indStyle == "indent" || indStyle == "both" {
		indent, _ = strconv.ParseInt(jobID, 10, 64)
		indentStr = strings.Repeat("-", int(indent%8)+4)
	}

	if terr != nil {
		log.Printf("[ERROR creating workdir (%s) for job %s trigger.]\n", terr, j.jobTag)
	} else {
		var workerOutputPath string
		var workerOutputFile *os.File
		consoleFName := "console.out"
		workerOutputPath = workDir + "/" + consoleFName
		workerOutputRelPath := fmt.Sprintf("%s/bacillus_%s_%s_%s/%s", jobHomeDir, j.jobOpts, j.jobTag, jobID, consoleFName)
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
		c.Env = append(c.Env, fmt.Sprintf("BACILLUS_JOBTAG=%s", j.jobTag))
		c.Env = append(c.Env, fmt.Sprintf("BACILLUS_WORKDIR=%s", workDir))
		c.Env = append(c.Env, fmt.Sprintf("BACILLUS_ARTFDIR=%s", fmt.Sprintf("%s/../../artifacts/bacillus_%s_%s_%s", workDir, j.jobOpts, j.jobTag, jobID)))
		c.Env = append(c.Env, j.jobEnv...)

		// JOB STATUS METADATA PREPENDED TO console.out
		// Job output status is encoded in first line of output log.
		// [1 2]
		//  1: state: r = running f = finished
		//  2: completion status: <n> = exit status, 0 = success; else failure
		//     status uses UNIX shell exit status convention (base 10 0-255))
		//
		// Line 2 is the relative path of the console.log file itself, used to
		// build a link to it for the /fullconsole/ endpoint link
		//
		// Line 3 is the JOBTAG of the job generating this console.out, used
		// by the top "/" endpoint to show recently active jobs (ie., those with
		// workdirs still present)
		//
		_, err := fmt.Fprintf(c.Stdout, "[r 255]\n")
		_, err = fmt.Fprintf(c.Stdout, "%s\n", strings.Replace(workerOutputRelPath, "workdir/", "/workdir/fullconsole/", 1))
		_, err = fmt.Fprintf(c.Stdout, "%s\n", j.jobTag)

		if err != nil {
			log.Fatal(err)
		}

		cerr := c.Start()
		if cerr != nil {
			log.Printf("[exec.Cmd: %+v]\n", c)
			j.w.WriteHeader(500)
			j.w.Write([]byte("ERR"))
			log.Printf("%s[ERROR on job %s trigger.]\n", indentStr,
				j.jobTag)
		} else {
			//runningJobCount += 1
			runningJobs[jobID] = j.jobTag
			jobCancellers[jobID] = cmdCancelFunc
			j.w.Write([]byte("OK"))
			log.Printf("<!--JOBID:%s:JOBID--><span style='background-color:%s'><a style='display:inline;' href='%s' title='Running'>[&acd;]</a>%s[job %s{%s}<a style='display:inline;' href='/cancel/?id=%s' title='Cancel'>[&cross;]</a> triggered.]</span>\n",
				jobID, instColour,
				workerOutputRelPath,
				indentStr,
				j.jobTag, jobID,
				jobID)
		}
		werr := c.Wait()
		//runningJobCount -= 1
		delete(runningJobs, jobID)

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

		if werr == nil {
			log.Printf("<!--JOBID:%s:JOBID--><span style='background-color:%s'><a href='%s' title='Done'>[&check;]</a>%s[job %s{%s}<a href='/artifacts/bacillus_%s_%s_%s/' title='Artifacts'>[&ccupssm;]</a> completed with status 0]</span><!--COMPLETION-->\n",
				jobID, instColour,
				workerOutputRelPath,
				indentStr,
				j.jobTag, jobID,
				j.jobOpts, j.jobTag, jobID)
		} else {
			log.Printf("<!--JOBID:%s:JOBID--><span style='background-color:%s'><span style='background-color:red'><a href='%s' title='Done With Errors'>[!]</a></span>%s[job %s{%s}<a href='/artifacts/bacillus_%s_%s_%s/' title='Partial Artifacts'>[&ccups;]</a> completed with error %s]</span><!--COMPLETION-->\n",
				jobID, instColour,
				workerOutputRelPath,
				indentStr,
				j.jobTag, jobID,
				j.jobOpts, j.jobTag, jobID,
				werr)
		}
	}
}

func jobCancelHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html")
	if !httpAuthSession(w, r) {
		return
	}

	v, ok := r.URL.Query()["id"]
	jobID := "undefined"
	if ok {
		jobID = v[0]
	}
	io.WriteString(w, `
					<html>
					<head>`+
		favIconHTML()+
		goBackJS("3000")+`
					</head>
					<body `+bodyBgndHTMLAttribs()+`>
					`)
	if jobCancellers[jobID] != nil {
		jobCancellers[jobID]()
		io.WriteString(w, fmt.Sprintf("<pre>Cancelled jobID %s</pre>\n", jobID))
	} else {
		io.WriteString(w, fmt.Sprintf("<pre>jobID %s already done or not found.</pre>\n", jobID))
	}
	io.WriteString(w, `
					</body>
					</html>`)
}

func launchJobListener(mainCtx context.Context, jobTag, jobOpts string, jobEnv []string, cmdMap map[string]string) {
	http.HandleFunc(fmt.Sprintf("/%s", jobTag),
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-type", "text/html")
			if !httpAuthSession(w, r) {
				return
			}
			io.WriteString(w, `
					<html>
					<head>`+
				favIconHTML()+
				goBackJS("3000")+`
					</head>
                    <body`+bodyBgndHTMLAttribs()+`>
					`)
			if shutdownModeActive {
				io.WriteString(w, fmt.Sprintf("<pre>Server is in shutdown mode, come back later.</pre>\n"))
				io.WriteString(w, `
					</body>
					</html>`)
				return
			}
			io.WriteString(w, fmt.Sprintf("<pre>Triggered %s</pre>\n", jobTag))
			io.WriteString(w, `
					</body>
					</html>`)

			go execJob(jobCtx{w, mainCtx, jobTag, jobOpts, jobEnv})
		})
}

func rootPageHandler(w http.ResponseWriter, r *http.Request) {
	// See if there are actions (currently just logout)
	_, ok := r.URL.Query()["logout"]
	if ok {
		w.Header().Set("Content-type", "text/html")
		w.Header().Set("WWW-Authenticate", `Basic realm="Bacillus"`)
		w.WriteHeader(http.StatusUnauthorized)
		//io.WriteString(w, `<head><meta http-equiv="refresh" content="0;URL='/'" /></head>`)
		io.WriteString(w, `<pre><a href='/'>You must log in.</a></pre>`)
		return
	}

	w.Header().Set("Content-type", "text/html")
	if !httpAuthSession(w, r) {
		return
	}

	io.WriteString(w, `
<html>
<head>`+
		favIconHTML()+
		/*refreshJS('r', "10")+*/
		xmlHTTPRequester("xhrLiveRunLogUpdate", "/api/lru?tl=5", `document.getElementById('liveRunLog').innerHTML = xhttp.response;`)+
		xmlHTTPRequester("xhrRunningJobsCount", "/api/rjc", `document.getElementById('liveRunLogCount').innerHTML = xhttp.response;`)+
		xhrlinkCSSFrag()+`
</head>
  <body `+bodyBgndHTMLAttribs()+`>
		`)
	io.WriteString(w, logoHdrHTML())
	io.WriteString(w, `
  <pre>
<a href='/runlog'>/runlog</a>: main log/activity view
<a href='/artifacts'>/artifacts</a>: where jobs (should) leave their stuff
  
Latest Job Activity (Running jobs:<span id='liveRunLogCount'>`+fmt.Sprintf("%d", len(runningJobs))+`</span>)
...
<span id='liveRunLog'>`+liveRunLogHTML(5)+`</span>
  LEGEND
  [&rtrif;] Start a job manually
  [&cross;] Cancel a running job
  [&ccupssm;] View completed job artifacts
  [&ccups;] View partial artifacts for a failed job
  [&acd;] Job is running - click to view
  [&check;] Job completed with OK(0) status - click to view
  <span style='background-color:red'>[!]</span> Job completed with nonzero status - click to view

  .. that's about it.
     Happy Build Automating, DevOps-ing, or whatever it's called these days...
	 
  Oh, and in case you need to...
  <a href='/shutdown'>halt any new jobs for a graceful shutdown</a>   (afterwards, use <strong>/rudeshutdown</strong>)
  <a href='/cancelshutdown'>cancel a planned shutdown</a>
`)
	if basicAuth {
		io.WriteString(w, `  <a href='`+logoutURI+`'>logout</a>
`)
	}
	io.WriteString(w, `
  Jobs Served (click Play to manually trigger)`+manualJobTriggersHTML(false)+`
  <span style='font-size: 8px; position: fixed; bottom: 0; right: 10;'><pre>Qui verifiers ratum efficiat? Non I.</pre></span>
  </pre>`)

	io.WriteString(w, manualJobTriggersJS())
	io.WriteString(w, `
</body>
</html>
	`)
}

// This hack is from https://stackoverflow.com/a/14329930/1012159
var logoutURI = `javascript:(function(c){var a,b="Logged out.";try{a=document.execCommand("ClearAuthenticationCache")}catch(d){}a||((a=window.XMLHttpRequest?new window.XMLHttpRequest:window.ActiveXObject?new ActiveXObject("Microsoft.XMLHTTP"):void 0)?(a.open("HEAD",c||location.href,!0,"logout",(new Date).getTime().toString()),a.send(""),a=1):a=void 0);a||(b="Your browser is too old or too weird to support log out functionality. Close all windows and restart the browser.");alert(b)})(/*pass safeLocation here if you need*/);`

//var logoutURI = `/?logout`

func cancelShutdownHandler(w http.ResponseWriter, r *http.Request) {
	//fmt.Println(r.URL)
	shutdownModeActive = false
	w.Header().Set("Content-type", "text/html")
	io.WriteString(w, `
					<html>
					<head>`+
		favIconHTML()+
		goBackJS("3000")+`
					</head>
                    <body`+bodyBgndHTMLAttribs()+`>
					`)
	io.WriteString(w, fmt.Sprintf("<pre>Shutdown mode off.</pre>\n"))
	io.WriteString(w, `
					</body>
					</html>`)
}

func shutdownHandler(w http.ResponseWriter, r *http.Request) {
	//fmt.Println(r.URL)
	shutdownModeActive = true
	w.Header().Set("Content-type", "text/html")
	io.WriteString(w, `
					<html>
					<head>`+
		favIconHTML()+
		goBackJS("3000")+`
					</head>
                    <body`+bodyBgndHTMLAttribs()+`>
					`)
	io.WriteString(w, fmt.Sprintf("<pre>Shutdown mode on. No new jobs can start.</pre>\n"))
	io.WriteString(w, `
					</body>
					</html>`)
}

func rudeShutdownHandler(w http.ResponseWriter, r *http.Request) {
	//fmt.Println(r.URL)
	w.Header().Set("Content-type", "text/html")
	io.WriteString(w, `
					<html>
					<head>`+
		favIconHTML()+`
					</head>
                    <body`+bodyBgndHTMLAttribs()+`>
					`)
	io.WriteString(w, fmt.Sprintf("<pre>.. so cold... so very, very cold..</pre>\n"))
	io.WriteString(w, `
					</body>
					</html>`)
	killSwitch <- true
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
			if strings.Count(fixed[idx], "<!--COMPLETION-->") != 0 {
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
					for seekIdx := idx - 1; seekIdx >= 0 && seekIdx > horizon; seekIdx-- {
						// NOTE we're modifying the 'live' view of
						// the logfile, not the direct data on disk, so
						// no need to replace byte-for-byte.
						// (If this func is optimized to be zero-copy
						//  however, it might need to be.)
						if strings.Contains(fixed[seekIdx], jobTag) {
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
	return fixed
}

func main() {
	killSwitch = make(chan bool, 1) // ensure a single send can proceed unblocked

	var createRunlog bool

	flag.StringVar(&addrPort, "a", ":9990", "[addr]:port on which to listen")
	flag.BoolVar(&basicAuth, "auth", true, "enable basic http auth login (be sure to also set -u and -p)")
	flag.StringVar(&strUser, "u", httpAuthUser, "web UI and endpoint username")
	flag.StringVar(&strPasswd, "p", httpAuthPasswd, "web UI and endpoint password")
	flag.BoolVar(&createRunlog, "c", false, "set true/1 to create new run.log, overwriting old one")
	flag.StringVar(&indStyle, "i", "both", "job entry indicator style [none|indent|colour|both]")
	flag.IntVar(&runLogTailLines, "rl", 30, "Scroll length of runlog (set to 0 for no limit)")
	flag.BoolVar(&attachStdout, "s", false, "set to true to see worker stdout/err if running in terminal")
	flag.Parse()

	mainCtx := context.Background()

	cmdMap = make(map[string]string)
	runningJobs = make(map[string]string)
	jobCancellers = make(map[string]func())

	var logfile *os.File
	var cerr error
	runLogFileName := fmt.Sprintf("run%s.log", strings.Split(addrPort, ":")[1])
	if !createRunlog {
		logfile, cerr = os.OpenFile(runLogFileName, os.O_RDWR, 0644)
	}
	if cerr != nil || createRunlog {
		logfile, _ = os.Create(runLogFileName)
	}

	log.SetOutput(logfile)
	log.Printf("[bacillus %s startup]\n", appVer)
	log.Printf("[listening on %s]\n", addrPort)

	//log.Printf("Registering handler for /runlog page.\n")
	http.HandleFunc("/runlog", runLogHandler)
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
			// We use _ as field separator for jobOpts, jobID in workdir/ and
			// artifacts/ dirs & job vars so they aren't allowed in the jobTag
			tag = strings.Replace(fields[0], "_", "-", -1)
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
			// Note presently only 'blind' hooks are supported
			// (ie., if webhook request contains POST JSON data,
			// it isn't read).
			launchJobListener(mainCtx, tag, jobOpts, jobEnv, cmdMap)
		}
	}

	log.Printf("--BACILLUS READY--\n")
	// Seek to end in case we're reusing this runlog to preserve previous
	// entries (yeah it's cheesy and probably error-prone if server was
	// killed during running jobs. Big deal, those entries
	// wouldn't show completion anyhow).
	logfile.Seek(0, 2)

	// Make a filesystem available for dir/file storage & retrieval by
	// jobs and devs. Jobs are responsible for its proper use.
	artifactBaseDir, aerr := filepath.Abs("artifacts")
	_ = artifactBaseDir
	if aerr == nil {
		http.Handle("/artifacts/",
			http.StripPrefix("/artifacts/",
				FileServer{Root: "/artifacts",
					Handler: http.FileServer(http.Dir("artifacts"))}))
	}

	http.Handle("/images/",
		http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))

	// Live runlog is just the tail of full runlog
	http.HandleFunc("/fullrunlog/", fullRunlogHandler)

	// Endpoint to cancel a job
	http.HandleFunc("/cancel/", jobCancelHandler)

	// A single endpoint handles the 'live' job output
	http.HandleFunc("/"+jobHomeDir+"/", consoleHandler)

	// Similarly, a single endpoint handles static full job output
	http.HandleFunc("/"+jobHomeDir+"/fullconsole/", fullConsoleHandler)

	// Enter shutdown mode (stop launching new jobs)
	http.HandleFunc("/shutdown", shutdownHandler)

	// Enter shutdown mode (stop launching new jobs)
	http.HandleFunc("/cancelshutdown", cancelShutdownHandler)

	// Rude exit (regardless of running jobs)
	http.HandleFunc("/rudeshutdown", rudeShutdownHandler)

	// Endpoint for XHR live run log updates
	http.HandleFunc("/api/lru", xhrLiveRunLogHandler)
	// Endpoint for XHR live run log updates
	http.HandleFunc("/api/rjc", xhrRunningJobsCountHandler)

	//// Logout sequence page
	//http.HandleFunc("/logout", logoutPageHandler)

	// And finally, the root fallback to give help on defined endpoints.
	http.HandleFunc("/", rootPageHandler)

	//go func() {
	//	log.Fatal(http.ListenAndServe(":9991", http.FileServer(http.Dir(jobHomeDir))))
	//}()

	// Rather than use http.ListenAndServe() we break out that func
	// and retain the http.Server var so we can call Shutdown() or
	// Close() if needed.
	server = &http.Server{Addr: addrPort, Handler: nil}
	go func() {
		log.Fatal(server.ListenAndServe())
	}()

	// .. and wait for a rude shutdown if requested
	_ = <-killSwitch
	server.Shutdown(nil)
}
