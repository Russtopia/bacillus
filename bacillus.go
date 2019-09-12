// bacillus - a A minimalist Build Automation/CI service
//
// bacillμs (Build Automation/Continuous Integration Low-Linecount μ(micro)-Service)
// listens for HTTP GET or POST events, executing specified actions on receipt of matching endpoint requests.
// Use it to respond to webhooks from SCM managers such as github, gitlab, gogs.io, etc.
// or from wget or curl requests made from git commit hooks.
//
// It is intended as a no-dependency, no-nonsense build automation system
// with minimal constraints so you may extend with whatever CI/CD/Devops process you want.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"sort"
	"time"

	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"blitter.com/go/brevity"
	"blitter.com/go/moonphase"
)

const (
	httpAuthUser   = "bacuser"
	httpAuthPasswd = "gramnegative" //b64:"YmFjdXNlcjpncmFtbmVnYXRpdmU="
	//indStyleNone          = "none"
	indStyleIndent = "indent"
	indStyleBoth   = "both"
	indStyleColour = "colour"
)

var (
	version   string
	gitCommit string

	server             *http.Server
	addrPort           string // eg. ":9990"
	basicAuth          bool   // flag: basic http auth
	strUser            string // API user
	strPasswd          string // API passwd
	attachStdout       bool
	shutdownModeActive bool
	killSwitch         chan bool
	//statUseUnicode  bool
	indStyle    string
	instCounter uint32
	//runningJobCount uint
	cmdMap               map[string]string
	runningJobs          runningJobList //map[string]string
	runningJobsLimit     uint           //max running jobs
	demoMode             bool           // set to true to disable /shutdown and /rudeshutdown
	jobHomeDir           string
	runLogTailLines      int
	showStagesOnFinished bool

	//checkSeq string
	//errSeq   string
	//playSeq  string

	// instColours is used for colouring job entry output if
	// enabled, to aid in visually matching launch/completion
	// entries
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

// runningJobInfo stores some essential bits about a
// running job so they can be cancelled, and to track
// stages of jobs if they update their _stage files
// key: jobID
type runningJobInfo struct {
	jobCanceller context.CancelFunc
	jobTag       string
	workDir      string
}

// runningJobList is the map of runningJobInfo entries
type runningJobList map[string]*runningJobInfo

// wrapper for io.WriteString() to ignore errors -- error-handling
// is useless to us for this application. Instead just log the error.
// Mostly to make gometalinter shut up.
func writeStr(w io.Writer, s string) {
	_, _ = io.WriteString(w, s) // nolint:errcheck
}

// There is a smattering of HTML and JS in this project, programmatically
// generated.
// No, I did not use templates.
// Yes, I may rewrite in the future to do so, but don't hold your breath.
// I didn't design this thing up-front, I wrote it to scratch an itch.
// That's what 'agile design' gets you :p

// xhrlinkCSSFrag emits CSS style used to mark job manual
// launch ('Play Job') entries in the dashboard.
func xhrlinkCSSFrag() string {
	return `<style>
		a.xhrlink {
			text-decoration: none;
			color: inherit;
		}
		a.xhrlink:visited {
			color: inherit;
		}
		a.xhrlink:hover {
			text-decoration: underline;
			background-color: aliceblue;
			cursor: pointer;
		}
		a.xhrlink:active {
			background-color: lightgreen;
		}
</style>
		`
}

// xmlHTTPRequester emits a JS function suitable for calling from an
// html element. Typically used for an onclick event to fire off an
// async GET request.
func xmlHTTPRequester(jsFuncName string, uri string, respHandlerJS string) string {
	return `
<!--!A<audio id='jobRunSound' type='audio/mpeg' src='audio/13280__schademans__pipe1.mp3'></audio>-->
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

// xhrRunningJobsCountHandler emits a string representation of the
// number of currently running jobs.
func xhrRunningJobsCountHandler(w http.ResponseWriter, r *http.Request) {
	writeStr(w, fmt.Sprintf("%d", len(runningJobs)))
}

// xhrLiveRunLogHandler emits an HTML fragment containing the specified
// number of runlog entries.
// params: r.URL.Query()["tl"][0] - uint, # of entries to yield
func xhrLiveRunLogHandler(w http.ResponseWriter, r *http.Request) {
	tl := 6
	v, ok := r.URL.Query()["tl"]
	if ok {
		fmt.Sscanf(v[0], "%d", &tl) // nolint:errcheck
	}
	writeStr(w, liveRunLogHTML(tl)) // nolint:errcheck
}

// favIconHTML emits an HTML fragment with the page's favIcon.
func favIconHTML() string {
	return `<link rel="icon" type="image/jpg" href="/images/logo.jpg"/>`
}

// logoShortHdrHTML emits an HTML fragment with the project logo/name/version
func logoShortHdrHTML() string {
	return `<img style='float:left;' width='16' src='/images/logo.jpg'/><pre><a href='/'>bacill&mu;s ` + version + `</a></pre>`
}

// logoShortHdrHTML emits an HTML fragment with the project logo/name/version
// and a link to the project's homepage.
func logoHdrHTML() string {
	return `<img style='float:left;' width='16' src='/images/logo.jpg'/><pre><a href='/'>bacill&mu;s ` + version + ` <a target='_' href='https://gogs.blitter.com/Russtopia/bacillus/src/master/README.md'>(What's this?)</a></pre>`
}

// bodyBgndHTMLAttribs emits an HTML fragment specifying the CSS background
// for the page.
func bodyBgndHTMLAttribs() string {
	if shutdownModeActive {
		return ` style='background: linear-gradient(to bottom, rgba(0,0,0,0.1) 0%,rgba(0,0,0,0.8) 100%); background-image: url("/images/bacillus-shutdown.jpg"); background-size: cover;'`
	}
	return ` style='background: linear-gradient(to bottom, rgba(0,0,0,0.1) 0%,rgba(0,0,0,0.8) 100%); background-image: url("/images/bacillus.jpg"); background-size: cover;'`
}

// goBackJS() returns a JS fragment to make a page go back after a
// specified delay.
func goBackJS(pages, ms string) string {
	return fmt.Sprintf(`
<script>
  // Go back after a short delay
  setInterval(function(){ /*window.location.href = document.referrer;*/ window.history.go(-%s); }, %s);
</script>
`, pages, ms)
}

// refreshMetaTag returns an HTML fragment defining the page refresh interval.
func refreshMetaTag(stat rune, intervalSecs string) string {
	if stat == 'r' {
		return `<meta http-equiv="refresh" content="` + intervalSecs + `">`
	}
	return ``
}

// forceReloadOnHistJS() emits a JS fragment suitable for inclusion into
// HTML page <head>ers that forces a page refresh if the page is visited
// via the browser history or back button.
func forceReloadOnHistJS() string {
	return `<script>
  if(performance.navigation.type == 2) {
    location.reload(true);
  }
  </script>
  `
}

// consActiveSpinnerCSS returns a CSS fragment defining the appearance
// and behaviour of a spinner if the enclosing page defines an element
// with ids #spinner, #finOKMarker and #finErrMarker.
func consActiveSpinnerCSS() string {
	return `
  <style>
    #spinner {
      position: fixed;
      right: 0.5em; bottom: 1em;
      font-family: monospace;
      margin: 0.5em;
      padding: 0.2em;
      font-size: 1.5em;
      font-weight: normal; //bold;
      background: skyblue;
      border: dotted 2px;
      border-radius: 0.5em;
    }
	
    #finOKMarker {
      position: fixed;
      right: 0.5em; bottom: 1em;
      font-family: monospace;
      margin: 0.5em;
      padding: 0.2em;
      font-size: 1.5em;
      font-weight: normal;
      background: lightgreen;
      border: dotted 2px;
      border-radius: 0.5em;
    }
	
    #finErrMarker {
      position: fixed;
      right: 0.5em; bottom: 1em;
      font-family: monospace;
      margin: 0.5em;
      padding: 0.2em;
      font-size: 1.5em;
      font-weight: bold;
      background: red;
      border: dotted 2px;
      border-radius: 0.5em;
    }
	
    //#stat {
    //  display: none;
    //}
  </style>
  `
}

// consActiveSpinnerJS returns JS code to animate a spinner element on
// the enclosing page, having a DOM id of #spinner.
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
	}
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

// compatJS emits a JS fragment to return the proper cross-browser version
// of document.scrollingElement, used to set the vertical scroll position
// of the current page.
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

// liveRunLogHTML returns the HTML for 'live' runlog with a specified # of
// tail lines, updated to reflect the current running status of pending jobs.
// The output is meant to be inserted within an enclosing, complete HTML page.
// (For just an HTML fragment of some specific # of most recent entries,
// to be inserted by client-side, see xurLiveRunLogHandler())
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
	tailLines = patchLiveViewOfRunLogEntries(tailLines, tl)

	if tl == 0 || tailCount < tl {
		ret += strings.Join(tailLines, "\n")
	} else {
		ret += strings.Join(tailLines[tailCount-tl:], "\n")
	}

	ret = strings.TrimPrefix(ret, "\n")
	ret = strings.TrimSuffix(ret, "\n")
	return
}

// manualJobTriggersJS returns a JS fragment for each defined job,
// meant to be bound to be onclick event of their corresponding
// 'Play Job' links in the dashboard or full runlog pages
// See manualJobTriggersHTML()
func manualJobTriggersJS() (ret string) {
	// sort the job keys
	keys := make([]string, len(cmdMap))
	for k := range cmdMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// For each endpoint 'fn', add a JS fragment which is a
	// function which will be bound elsewhere to the onclick
	// event of the job's link in the manual job trigger section
	// of the page: see manualJobTriggersHTML().
	for _, k := range keys {
		if len(cmdMap[k]) > 0 {
			fn := strings.Replace(k, "-", "", -1)
			ret += xmlHTTPRequester(fn, k, "")
			ret += `<script>
			setInterval( xhrLiveRunLogUpdate, 2000 );
			setInterval( xhrRunningJobsCount, 2000 );
			</script>`
		}
	}
	return
}

func hasParameterSpecifier(line string) bool {
	if strings.HasPrefix(line, "#-?") ||
		strings.HasPrefix(line, "/*-?") ||
		strings.HasPrefix(line, "//*-?") {
		return true
	}
	return false
}

// isParameterizedBuildScript scans job script scriptFName for build
// parameter form specifiers.
// It returns false if there are none, otherwise true
func isParameterizedBuildScript(scriptFName string) bool {
	isParamJob := false
	fileBytes, e := ioutil.ReadFile(jobHomeDir + strings.TrimPrefix(scriptFName, ".."))
	if e != nil {
		fmt.Println("Error:", e)
		return false
	}
	lines := strings.Split(string(fileBytes), "\n")
	for _, line := range lines {
		if hasParameterSpecifier(line) {
			isParamJob = true
		} else if isParamJob {
			break
		}
	}
	return isParamJob
}

// genParameterizedBuildForm scans job script scriptFName for build parameter
// form specifiers, returning an HTML fragment suitable for setting all
// defined parameters.
//
// nolint:gocyclo
func genParameterizedBuildForm(jobTag, scriptFName string) (ret string) {
	paramJobLine := false

	scriptFName = strings.TrimPrefix(scriptFName, "../")
	fileBytes, e := ioutil.ReadFile(jobHomeDir + "/" + scriptFName)
	if e != nil {
		fmt.Println("Error:", e)
		return
	}
	lines := strings.Split(string(fileBytes), "\n")
	for _, line := range lines {
		// TODO: parse lines for "#-?" entries, build
		// HTML page w/form to set params and pass to job
		// via a submit link
		if hasParameterSpecifier(line) {
			if !paramJobLine {
				// First entry, build form prologue

				// The hidden ?paramSet will trigger
				// the final stage of same endpoint that
				// calls this func (launchJob)
				//
				// TODO: form action="%s"
				ret += `
				<h2>` + jobTag + `</h2>
				<h3>Build with Parameters </h3>
				<hr />
				<form action="/` + jobTag + `?paramSet" method="GET">
				<input type="hidden" name="paramSet" />
				`
			}
			paramJobLine = true

			// Determine type of build param
			// [0]:paramMarker (#-?) [1]:type (b|c|s) [2]:name [3]:(vals ...)
			paramFields := strings.Split(line, "?")
			var paramComment string
			if len(paramFields) > 4 {
				// has comment
				paramComment = paramFields[4]
			}

			switch paramFields[1] {
			case "s":
				ret += fmt.Sprintf("%s:<input type='text' name='%s' value='%s' />&nbsp;%s<br />\n",
					paramFields[2], paramFields[2], paramFields[3],
					paramComment)
			case "c":
				choices := strings.Split(paramFields[3], "|")
				ret += paramFields[2] + ":<select name='" + paramFields[2] + "'>\n"
				for _, c := range choices {
					ret += "  <option value='" + c + "'>" + c + "</option>\n"
				}
				ret += "</select>&nbsp;" + paramComment + "<br />\n"
			case "b":
				// NOTE the 'b' bool type uses HTML input type='checkbox'
				// which sends nothing if unset. (eg., the job should
				// expect a missing param and assume that means 'false',
				// 'off', 'disabled' ...
				//
				// In bash syntax that would typically be handled like:
				// option=${option:-"false"}

				ret += paramFields[2]
				ret += "  <input type='checkbox' name='" + paramFields[2] + "'"
				if paramFields[3] == "on" ||
					paramFields[3] == "true" ||
					paramFields[3] == "1" ||
					strings.HasPrefix(paramFields[3], "enable") {
					ret += "value='true' checked"
				} // else {
				//	ret += "value='false'"
				//}
				ret += "/>&nbsp;" + paramComment + "<br />\n"
			}
		} else if paramJobLine {
			// End of param specifiers, emit form epilogue
			ret += `
				<input type="submit" value="Build"/>
				</form>`
			break
		}
	}
	return ret
}

func sayingFooterHTML() (ret string) {
	prefix := `<pre style='font-size: 8px; position: fixed; bottom: 0px; right: 10px;'>`
	suffix := `</pre>`
	t := time.Now()
	m := moonphase.New(t)
	n := m.PhaseSymbol()
	footerMain := ""
	switch n {
	case "New Moon":
		footerMain = n + " It is pitch dark. You are likely to be eaten by a Grue."
	case "Full Moon":
		footerMain = n + " Watch out! Full moon tonight."
	default:
		footerMain = fmt.Sprintf("%s ", n) + `Best viewed using DejaVu font family --- Qui verifiers ratum efficiat? Non I.`
	}

	ret = prefix + footerMain + suffix
	return
}

// manualJobTriggersHTML returns an HTML fragment containing href links to
// all defined job endpoints. Note manualJobTriggersJS() must be used
// in conjunction with this output to bind onclick handlers to them.
func manualJobTriggersHTML(fullLogLink bool) (ret string) {
	ret = "<pre style='background-color: skyblue;'>"
	keys := make([]string, len(cmdMap))
	for k := range cmdMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if len(cmdMap[k]) > 0 {
			// ===================
			// Examine script at (k) for job-param syntax:
			// If present, gen code to go to a params page dynamically
			// constructed w/param form, rather than a direct XHR to
			// launch endpoint
			// ===================
			if _, e := os.Stat(strings.Replace(cmdMap[k], "..", jobHomeDir, -1)); e != nil {
				ret += fmt.Sprintf("-- job script %s not found --\n", cmdMap[k])
			} else {
				if isParameterizedBuildScript(cmdMap[k]) {
					ret += fmt.Sprintf("<a class='xhrlink' title='Play Job with Parameters' href='%s?param'>[&rtri;] %s [action %s]</a>\n",
						k, k, cmdMap[k])
				} else {
					fn := strings.Replace(k, "-", "", -1)
					ret += fmt.Sprintf(`<a class='xhrlink' onclick='%s(); return false;' title='Play Job' href='%s'>[&rtrif;] %s [action %s]</a>`+"\n",
						fn, k, k, cmdMap[k])
				}
			}
		}
	}
	if fullLogLink {
		ret += "<a href='/fullrunlog'>... click for full runlog ...</a>"
	}
	ret += "</pre>"
	return
}

// httpAuthSession should be used at the start of all endpoints to
// enforce basic HTTP auth (this function is a nop if auth is disabled
// in server config). Returns true if user is authorized, else
// a 'Not logged in' page is sent to the client and false is returned.
//
// NOTE basic auth is not secure by itself; the user/password are
// sent to the server in plaintext unless TLS protects the server.
// A reverse proxy enforcing HTTPS on the server is highly recommended.
func httpAuthSession(w http.ResponseWriter, r *http.Request) (auth bool) {
	w.Header().Set("Cache-Control", "no-cache")

	if !basicAuth {
		return true
	}

	u, p, ok := r.BasicAuth()
	if ok && u == strUser && p == strPasswd {
		return true
	}

	w.Header().Set("WWW-Authenticate", `Basic realm="Bacillus"`)
	w.WriteHeader(http.StatusUnauthorized)
	writeStr(w, "Not logged in.") // nolint:errcheck
	return
}

// TODO: types for matching JSON events of
// supported webhooks: gogs.io, github, gitlab, ... ?
// For now, the 'blind' endpoint is the only one supported,
// meaning the request can't communicate any extra data to the
// job invocation in a GET or POST request.

func runLogHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html;charset=UTF-8")
	if !httpAuthSession(w, r) {
		return
	}

	writeStr(w, `
<html>
<head>`+
		favIconHTML()+
		xhrlinkCSSFrag()+
		xmlHTTPRequester("xhrLiveRunLogUpdate", fmt.Sprintf("/api/lru?tl=%d", runLogTailLines), `document.getElementById('liveRunLog').innerHTML = xhttp.response;`)+
		logoShortHdrHTML()+`
</head>
<body `+bodyBgndHTMLAttribs()+`>`)
	writeStr(w, manualJobTriggersHTML(true)+
		`<pre id='liveRunLog'>`+liveRunLogHTML(runLogTailLines)+`</pre>`)

	writeStr(w, manualJobTriggersJS())
	writeStr(w, `
</body>
</html>
    `)
}

func fullRunlogHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html;charset=UTF-8")
	if !httpAuthSession(w, r) {
		return
	}

	runLog, e := ioutil.ReadFile(fmt.Sprintf("run%s.log", strings.Split(addrPort, ":")[1]))
	if e != nil {
		writeStr(w, fmt.Sprintf("%s", e))
		return
	}
	writeStr(w, `
<html>
<head>`+
		favIconHTML()+
		logoShortHdrHTML()+`
</head>
<body>`) // nolint:errcheck

	writeStr(w, `
<pre>
`+string(runLog)+`</pre>
</body>
</html>`) // nolint:errcheck
}

func fullConsoleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/plain;charset=UTF-8")
	if !httpAuthSession(w, r) {
		return
	}

	consoleLog, e := ioutil.ReadFile(strings.Replace(r.URL.String()[1:], "/fullconsole", "", 1))
	if e != nil {
		writeStr(w, fmt.Sprintf("%s", e))
		return
	}
	writeStr(w, string(consoleLog))
}

func consoleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html;charset=UTF-8")
	if !httpAuthSession(w, r) {
		return
	}

	// Read file from URL, removing leading / as workdir is rel to us
	consoleLog, e := ioutil.ReadFile(r.URL.String()[1:])
	if e != nil {
		writeStr(w, fmt.Sprintf("%s", e))
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
	n, _ := fmt.Sscanf(consStat, "[%c %03d]", &stat, &code)
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

	writeStr(w, `
<html>
<head>
`+
		favIconHTML()+
		refreshMetaTag(stat, "5")+
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

	writeStr(w, logoShortHdrHTML())
	writeStr(w, "<pre>")
	writeStr(w, string(consoleLog))
	writeStr(w, "\n</pre>")

	writeStr(w, "<pre>"+fmt.Sprintln(r.URL)+"</pre>")
	writeStr(w, `
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

type hookEvt struct {
	Ref         string `json:"ref"`
	Before      string `json:"before"`
	After       string `json:"after"`
	Compare_url string `json:"compare_url"`
	Commits     []struct {
		Id      string `json:"id"`
		Message string `json:"message"`
		Url     string `json:"url"`
		Author  struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username"`
		}
	}
}

// execJob spawns the actual job, waiting for it to complete and
// marks the runlog entry to indicate completion status and supply
// the artifact link.
//
//nolint:gocyclo
func execJob(j jobCtx, hookData hookEvt) {
	// Some wrinkles in the exec.Command API: If there are no args,
	// one must completely omit the args ... to avoid strange errors
	// with some commands that see a blank "" arg and complain.
	cmd := strings.Split(cmdMap[j.jobTag], " ")[0]
	cmdStrList := strings.Split(cmdMap[j.jobTag], " ")[1:]
	//fmt.Printf("%s %v\n", cmd, cmdStrList)
	cmdCancelCtx, cmdCancelFunc := context.WithCancel(j.mainCtx)
	defer cmdCancelFunc()

	var c *exec.Cmd
	if len(cmdStrList) > 0 {
		c = exec.CommandContext(cmdCancelCtx, cmd, strings.Join(cmdStrList, " "))
	} else {
		c = exec.CommandContext(cmdCancelCtx, cmd)
	}

	var instColourIdx uint32
	if indStyle == indStyleColour || indStyle == indStyleBoth {
		instColourIdx = rand.Uint32() % uint32(len(instColours))
		instCounter++
	} else {
		instColourIdx = 0
	}
	instColour := instColours[instColourIdx]

	dirTmp, _ := filepath.Abs(jobHomeDir)
	workDir, terr := ioutil.TempDir(dirTmp, fmt.Sprintf("bacillus_%s_%s_", j.jobOpts, j.jobTag))
	c.Dir = workDir
	jobID := workDir[strings.LastIndex(workDir, "_")+1:]
	var indent int64
	var indentStr string
	if indStyle == indStyleIndent || indStyle == indStyleBoth {
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
		workerOutputRelPath := fmt.Sprintf("%s/bacillus_%s_%s_%s/%s",
			jobHomeDir,
			j.jobOpts,
			j.jobTag,
			jobID,
			consoleFName)
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
		// Extra environment is provided by webhooks, so we'll set those
		// separately if JSON is present.
		// It is the job script's responsibility to check for these and,
		// if not set, default sensibly (eg., if the push was done by a
		// raw git post-receive hook rather than a webhook, there will be
		// no BACILLUS_REF; script usually should default to "refs/master")
		if hookData.Ref != "" {
			c.Env = append(c.Env, fmt.Sprintf("BACILLUS_REF=%s", strings.TrimPrefix(hookData.Ref, "heads/")))
		}
		if len(hookData.Commits) > 0 {
			c.Env = append(c.Env, fmt.Sprintf("BACILLUS_COMMITID=%s", hookData.Commits[0].Id))
		}

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
		fmt.Fprintf(c.Stdout, "[r 255]\n") //nolint:errcheck
		fmt.Fprintf(c.Stdout, "%s\n",
			strings.Replace(workerOutputRelPath, jobHomeDir, "/"+jobHomeDir+"/fullconsole", 1)) //nolint:errcheck
		fmt.Fprintf(c.Stdout, "%s\n", j.jobTag) //nolint:errcheck

		cerr := c.Start()
		if cerr != nil {
			log.Printf("[exec.Cmd: %+v]\n", c)
			j.w.WriteHeader(500)
			writeStr(j.w, "ERR")
			log.Printf("%s[ERROR on job %s trigger.]\n", indentStr,
				j.jobTag)
		} else {
			if len(runningJobs) >= int(runningJobsLimit) {
				writeStr(j.w, "WHOA BESSIE")
				log.Printf("<!--JOBID:%s:JOBID--><span style='background-color:grey'><span style='background-color:maroon'>XXX</span>%s[%s not launched: running job limit reached]</span><!--COMPLETION-->\n",
					jobID,
					"", /*indentStr,*/
					j.jobTag)
				return
			}

			runningJobs[jobID] = &runningJobInfo{
				jobCanceller: cmdCancelFunc, jobTag: j.jobTag, workDir: workDir}

			writeStr(j.w, "OK")
			log.Printf("<!--JOBID:%s:JOBID-->"+
				"<span style='background-color:%s'><a style='display:inline;' href='%s' title='Running'>[&acd;]</a>%s[%s{%s}<a style='display:inline;' href='/cancel/?id=%s' title='Cancel'>[&cross;]</a> triggered.]<!--:STAGE:--></span>\n",
				jobID, instColour,
				workerOutputRelPath,
				indentStr,
				j.jobTag, jobID,
				jobID)
		}
		werr := c.Wait()

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
				fmt.Fprintf(workerOutputFile, "[f %03d]", int8(exitStatus)) //nolint:errcheck
				//log.Print(c.Stderr /*stdErrBuffer*/)
				//log.Printf("%s[Exit Status: %d]\n", indentStr, int32(exitStatus)) //#
			}
		} else {
			// exec.Cmd automatically closes its files on exit, so we need to
			// reopen here to write the status at offset 0
			workerOutputFile, _ = os.OpenFile(workerOutputPath, os.O_RDWR, 0777)
			fmt.Fprintf(workerOutputFile, "[f %03d]", 0) //nolint:errcheck
			//workerOutputFile.WriteAt([]byte(fmt.Sprintf("[f %03d]", 0)), 0)
		}

		stageStr := ""
		if showStagesOnFinished {
			currentStage, e := ioutil.ReadFile(runningJobs[jobID].workDir + "/_stage")
			stageStr = string(currentStage)
			if e == nil {
				stageStr = " |" +
					strings.TrimSpace(strings.Replace(brevity.PreEllipse(stageStr, ":", 3), ":", " &compfn; ", -1)) +
					"|"
			} else {
				stageStr = "|???|"
			}
		}

		if werr == nil {
			log.Printf("<!--JOBID:%s:JOBID--><span style='background-color:%s'><a href='%s' title='Done'>[&check;]</a>%s[%s{%s}<a href='/artifacts/bacillus_%s_%s_%s/' title='Artifacts'>[&ccupssm;]</a> completed with status 0]%s</span><!--COMPLETION-->\n",
				jobID, instColour,
				workerOutputRelPath,
				indentStr,
				j.jobTag, jobID,
				j.jobOpts, j.jobTag, jobID,
				stageStr)
		} else {
			log.Printf("<!--JOBID:%s:JOBID--><span style='background-color:%s'><span style='background-color:red'><a href='%s' title='Done With Errors'>[!]</a></span>%s[%s{%s}<a href='/artifacts/bacillus_%s_%s_%s/' title='Partial Artifacts'>[&ccups;]</a> completed with error %s]%s</span><!--COMPLETION-->\n",
				jobID, instColour,
				workerOutputRelPath,
				indentStr,
				j.jobTag, jobID,
				j.jobOpts, j.jobTag, jobID,
				werr,
				stageStr)
		}
		delete(runningJobs, jobID)
	}
}

func jobCancelHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html;charset=UTF-8")
	if !httpAuthSession(w, r) {
		return
	}

	v, ok := r.URL.Query()["id"]
	jobID := "undefined"
	if ok {
		jobID = v[0]
	}
	writeStr(w, `
					<html>
					<head>`+
		favIconHTML()+
		goBackJS("1", "3000")+`
					</head>
					<body `+bodyBgndHTMLAttribs()+`>
					`)
	if runningJobs[jobID] != nil && runningJobs[jobID].jobCanceller != nil {
		runningJobs[jobID].jobCanceller()
		writeStr(w, fmt.Sprintf("<pre>Cancelled jobID %s</pre>\n", jobID))
	} else {
		writeStr(w, fmt.Sprintf("<pre>jobID %s already done or not found.</pre>\n", jobID))
	}
	writeStr(w, `
					</body>
					</html>`)
}

// Launch a job listener endpoint, which in the case of simple jobs
// directly calls the job, or, in the case of parameterized jobs,
// dynamically builds and emits a page containing an HTML form with which
// the user may set job parameters before submitting to launch the job.
//
// The HTML emitted is selected by whether the visiting URL has
// no parameters, ?param or ?usingParams: no parameters directly launches a job,
// ?param emits an HTML form page, and ?usingParams launches the job with the
// submitted parameters from the ?param form page.
func launchJobListener(mainCtx context.Context, cmd, jobTag, jobOpts string, jobEnv []string, cmdMap map[string]string) {
	origJobEnv := jobEnv // saved to reset the env on each invocation

	http.HandleFunc(fmt.Sprintf("/%s", jobTag),
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-type", "text/html;charset=UTF-8")

			jobEnv = origJobEnv // reset each time invoked, we append to it

			if !httpAuthSession(w, r) {
				return
			}

			// Check if JSON is present (from a webhook)
			decoder := json.NewDecoder(r.Body)
			var hookData = hookEvt{}
			jsonErr := decoder.Decode(&hookData)
			_ = jsonErr
			// Depending on whether the page being emitted is ?param (form)
			// or ?usingParams (form submission/job launch), set how many
			// pages the launch confirmation page needs to jump back
			// to return to the dashboard or runlog page.
			var pagesBack string
			_, ok := r.URL.Query()["usingParams"]
			if ok {
				pagesBack = "2"
			} else {
				pagesBack = "1"
			}

			headerFragS := "<html><head>" + favIconHTML() + logoShortHdrHTML()
			headerFragM := goBackJS(pagesBack, "3000")
			headerFragE := "</head>"
			bodyFragB := "<body " + bodyBgndHTMLAttribs() + ">"
			bodyFragM := ""
			bodyFragE := "</body></html>"

			_, ok = r.URL.Query()["param"]
			if ok {
				headerFragM = ""
			}

			writeStr(w, headerFragS+headerFragM+headerFragE)
			writeStr(w, bodyFragB)

			if shutdownModeActive {
				bodyFragM = fmt.Sprintf("<pre>Server is in shutdown mode, come back later.</pre>\n")
				bodyFragM += goBackJS(pagesBack, "3000")
			} else if _, ok := r.URL.Query()["param"]; ok {
				// Get job-defined parameter form
				bodyFragM = genParameterizedBuildForm(jobTag, cmd)
			} else if _, ok = r.URL.Query()["usingParams"]; ok {
				// If we're called back with ?usingParams, which is submitted
				// form data from dynamically-generated ?param form above,
				// parse those values from r.URL.Query(), adding to jobEnv[].
				r.ParseForm() //nolint:errcheck
				for k, v := range r.Form {
					if len(v) > 0 {
						jobEnv = append(jobEnv, k+`="`+v[0]+`"`)
					}
				}
				bodyFragM = fmt.Sprintf("<pre>Triggered parameterized build %s</pre>\n", jobTag)
				// Launch parameterized job
				go execJob(jobCtx{w, mainCtx, jobTag, jobOpts, jobEnv}, hookEvt{})

				//!A writeStr(w, `<audio id='jobRunSound' type='audio/mpeg' src='audio/13280__schademans__pipe1.mp3'></audio>`)
				//!A writeStr(w, `<script>document.getElementById("jobRunSound").play();</script>`)
			} else {
				// Launch simple job
				bodyFragM = fmt.Sprintf("<pre>Triggered %s</pre>\n", jobTag)
				go execJob(jobCtx{w, mainCtx, jobTag, jobOpts, jobEnv}, hookData)
				//!A writeStr(w, `<audio id='jobRunSound' type='audio/mpeg' src='audio/13280__schademans__pipe1.mp3'></audio>`)
				//!A writeStr(w, `<script>document.getElementById("jobRunSound").play();</script>`)
			}
			fmt.Fprintf(w, bodyFragM+bodyFragE) // nolint:errcheck
		})
}

// rootPageHandler serves the 'root' ('main' or 'dashboard') page.
func rootPageHandler(w http.ResponseWriter, r *http.Request) {
	// See if there are actions (currently just logout)
	_, ok := r.URL.Query()["logout"]
	if ok {
		w.Header().Set("Content-type", "text/html;charset=UTF-8")
		w.Header().Set("WWW-Authenticate", `Basic realm="Bacillus"`)
		w.WriteHeader(http.StatusUnauthorized)
		//writeStr(w, `<head><meta http-equiv="refresh" content="0;URL='/'" /></head>`)
		writeStr(w, `<pre><a href='/'>You must log in.</a></pre>`)
		return
	}

	w.Header().Set("Content-type", "text/html;charset=UTF-8")
	if !httpAuthSession(w, r) {
		return
	}

	writeStr(w, `
<html>
<head>`+
		favIconHTML()+
		forceReloadOnHistJS()+
		/*refreshMetaTag('r', "10")+*/
		xmlHTTPRequester("xhrLiveRunLogUpdate", "/api/lru?tl=6", `document.getElementById('liveRunLog').innerHTML = xhttp.response;`)+
		xmlHTTPRequester("xhrRunningJobsCount", "/api/rjc", `document.getElementById('liveRunLogCount').innerHTML = xhttp.response;`)+
		xhrlinkCSSFrag()+`
</head>
  <body `+bodyBgndHTMLAttribs()+`>
		`)
	writeStr(w, logoHdrHTML())
	//!A writeStr(w, "<audio id='jobRunSound' type='audio/mpeg' src='audio/13280__schademans__pipe1.mp3'></audio>")
	writeStr(w, `
  <pre>
<a href='/runlog'>/runlog</a>: main log/activity view
<a href='/artifacts'>/artifacts</a>: where jobs (should) leave their stuff
  
Latest Job Activity (Running jobs:<span id='liveRunLogCount'>`+fmt.Sprintf("%d", len(runningJobs))+`</span> Max `+fmt.Sprintf("%d", runningJobsLimit)+`)
...
<span id='liveRunLog'>`+liveRunLogHTML(6)+`</span>

  LEGEND
  [&rtrif;] Start a job manually
  [&rtri;] Start a job with parameters
  [&cross;] Cancel a running job
  [&ccupssm;] View completed job artifacts
  [&ccups;] View partial artifacts for a failed job
  [<img style='border:none; border-width:0px; width:0.8em; margin:0px; padding:0px;' src='images/run-throbber.gif'/>] Job is running - click to view
  [&check;] Job completed with OK(0) status - click to view
  <span style='background-color:red'>[!]</span> Job completed with nonzero status - click to view

  .. that's <a class="xhrlink" style="text-decoration:none" href="/about">about</a> it.
	 
  Oh, and in case you need to...
  <a href='/shutdown'>prevent any new jobs for a graceful shutdown</a>   (afterwards, use <strong>/rudeshutdown</strong>)
  <a href='/cancelshutdown'>cancel a planned shutdown</a>
`)
	if basicAuth {
		writeStr(w, `  <a href='`+logoutURI+`'>logout</a>
`)
	}
	writeStr(w, `
Jobs Served (click Play to manually trigger)`+
		manualJobTriggersHTML(false)+
		sayingFooterHTML())

	writeStr(w, manualJobTriggersJS())
	writeStr(w, `
</body>
</html>
	`)
}

// Perform a logout from HTTP Basic Auth
//
// Apparently it is quite difficult to clean out HTTP basic auth in modern
// browsers.
//
// This hack is from https://stackoverflow.com/a/14329930/1012159
var logoutURI = `javascript:(function(c){var a,b="Logged out.";try{a=document.execCommand("ClearAuthenticationCache")}catch(d){}a||((a=window.XMLHttpRequest?new window.XMLHttpRequest:window.ActiveXObject?new ActiveXObject("Microsoft.XMLHTTP"):void 0)?(a.open("HEAD",c||location.href,!0,"logout",(new Date).getTime().toString()),a.send(""),a=1):a=void 0);a||(b="Your browser is too old or too weird to support log out functionality. Close all windows and restart the browser.");alert(b)})(/*pass safeLocation here if you need*/);`

//var logoutURI = `/?logout`

// cancelShutdownHandler .. does what you'd expect, cancels a planned
// server shutdown.
//
// NOTE the server does not itself shutdown after scheduling one without
// explicit admin action, by killing the process or manually visiting
// the /rudeshutdown URI endpoint.
func cancelShutdownHandler(w http.ResponseWriter, r *http.Request) {
	//fmt.Println(r.URL)
	shutdownModeActive = false
	w.Header().Set("Content-type", "text/html;charset=UTF-8")
	writeStr(w, `
					<html>
					<head>`+
		favIconHTML()+
		goBackJS("1", "3000")+`
					</head>
                    <body`+bodyBgndHTMLAttribs()+`>
					`)
	if demoMode {
		writeStr(w, fmt.Sprintf("<pre>Shutdown mode disabled by admin.</pre>\n"))
	} else {
		writeStr(w, fmt.Sprintf("<pre>Shutdown mode off.</pre>\n"))
	}
	writeStr(w, `
					</body>
					</html>`)
}

// shutdownHandler puts the server into shutdown mode: refuse to start
// any new jobs until the /cancelshutdown endpoint is visited, or
// the admin kills the server or visits /rudeshutdown to tell it to exit.
func shutdownHandler(w http.ResponseWriter, r *http.Request) {
	//fmt.Println(r.URL)
	w.Header().Set("Content-type", "text/html;charset=UTF-8")
	writeStr(w, `
					<html>
					<head>`+
		favIconHTML()+
		goBackJS("1", "3000")+`
					</head>
                    <body`+bodyBgndHTMLAttribs()+`>
					`)
	if demoMode {
		writeStr(w, fmt.Sprintf("<pre>Shutdown mode disabled by admin.</pre>\n"))
	} else {
		shutdownModeActive = true
		writeStr(w, fmt.Sprintf("<pre>Shutdown mode on. No new jobs can start.</pre>\n"))
	}
	writeStr(w, `
					</body>
					</html>`)
}

// aboutPageHandler displays author/license information.
func aboutPageHandler(w http.ResponseWriter, r *http.Request) {
	if !httpAuthSession(w, r) {
		return
	}

	writeStr(w, `
<html>
<head>`+
		favIconHTML()+
		//xhrlinkCSSFrag()+
		//xmlHTTPRequester("xhrLiveRunLogUpdate", fmt.Sprintf("/api/lru?tl=%d", runLogTailLines), `document.getElementById('liveRunLog').innerHTML = xhttp.response;`)+
		logoShortHdrHTML()+`
</head>
<body `+bodyBgndHTMLAttribs()+`>`)
	writeStr(w, `<p><img src="images/BenderCI.jpg" width="600"/></p>`)
	writeStr(w, goBackJS("1", "10000"))
	writeStr(w, `<pre>
  bacill&mu;s CI server. Written in <a href="https://golang.org/">Go</a>
  &copy; Copyright 2019 by Russ Magee. All Rights Reserved.
</pre>
</body>
</html>
    `)

}

// rudeShutdownHandler tells the server to exit. The /shutdown endpoint
// should be visited and, if possible, running jobs be allowed to finish
// before using this endpoint.
func rudeShutdownHandler(w http.ResponseWriter, r *http.Request) {
	//fmt.Println(r.URL)
	w.Header().Set("Content-type", "text/html;charset=UTF-8")
	writeStr(w, `
					<html>
					<head>`+
		favIconHTML()+`
					</head>
                    <body`+bodyBgndHTMLAttribs()+`>
					`)
	if demoMode {
		writeStr(w, fmt.Sprintf("<pre>.. rudeshutdown disabled by admin.</pre>\n"))
		writeStr(w, `
					</body>
					</html>`)
	} else {
		writeStr(w, fmt.Sprintf("<pre>.. so cold... so very, very cold..</pre>\n"))
		writeStr(w, `
					</body>
					</html>`)
		killSwitch <- true
	}
}

// patchLiveRunEntries looks at a limited back-history of runlog entries
// and updates ones representing currently-running jobs with live status.
//
// HTML comment blocks inside elements, <!--JOBID:>...<:JOBID--> and <!--:STAGE:-->
// are used to find elements to patch with live run status info.
// Recently-completed jobs also have their in-progress symbol at the start
// replaced if the placeholder comment <!--COMPLETION--> is found in a
// later log entry.
//
//nolint:gocyclo
func patchLiveRunEntries(idx, horizon int, fixed []string) []string {
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
					if indStyle == indStyleBoth || indStyle == indStyleIndent {
						fixed[seekIdx] = strings.Replace(fixed[seekIdx],
							"---", "------", 1)
					} else if indStyle == "colour" {
						fixed[seekIdx] = strings.Replace(fixed[seekIdx],
							"[job", "   [job", 1)
					}
				}
			}
		}
	} else if strings.Contains(fixed[idx], "<!--:STAGE:-->") &&
		strings.Contains(fixed[idx], "[&acd;]") {
		fixed[idx] = strings.Replace(fixed[idx], "&acd;", "<img style='border:none; border-width:0px; width:0.8em; margin:0px; padding:0px;' src='images/run-throbber.gif'/>", 1)
		// Found an fixed[idx] for a running job;
		// fetch the stage, if defined, and add it to the
		// live line's view.
		var jidStart, jidEnd int
		var jobID string
		jidStart = strings.Index(fixed[idx], "<!--JOBID:")
		if jidStart != -1 {
			jidStart += len("<!--JOBID:")
			jidEnd = strings.Index(fixed[idx], ":JOBID-->")
		}
		if jidStart != -1 && jidEnd != -1 {
			jobID = fixed[idx][jidStart:jidEnd]
		}
		if runningJobs[jobID] != nil {
			currentStage, e := ioutil.ReadFile(runningJobs[jobID].workDir + "/_stage")
			if e == nil {
				stageStr := brevity.PreEllipse(string(currentStage), ":", 3)
				fixed[idx] = strings.Replace(fixed[idx], "<!--:STAGE:-->",
					" |<strong>"+
						strings.TrimSpace(strings.Replace(stageStr, ":", " &compfn; ", -1))+
						"<img style='border:none; border-width:0px; width:0.8em; margin:0px; padding:0px; padding-left:1px;' src='images/stage-throbber.gif'/>"+
						"</strong>|", 1)
			}
		}
	}
	return fixed
}

func patchLiveViewOfRunLogEntries(orig []string, horizon int) (fixed []string) {
	//FIXME: There is definitely a less copy-intensive way to do this.
	//
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
			fixed = patchLiveRunEntries(idx, horizon, fixed)
		}
	}
	return fixed
}

func main() {
	var createRunlog bool

	flag.StringVar(&addrPort, "a", ":9990", "[addr]:port on which to listen")
	flag.BoolVar(&basicAuth, "auth", true, "enable basic http auth login (be sure to also set -u and -p)")
	flag.StringVar(&strUser, "u", httpAuthUser, "web UI and endpoint username")
	flag.StringVar(&strPasswd, "p", httpAuthPasswd, "web UI and endpoint password")
	flag.StringVar(&jobHomeDir, "w", "workdir", "workdir for jobs (relative to bacillus launch dir)")
	flag.BoolVar(&createRunlog, "c", false, "set true/1 to create new run.log, overwriting old one")
	flag.StringVar(&indStyle, "i", indStyleBoth, "job entry indicator style [none|indent|colour|both]")
	flag.IntVar(&runLogTailLines, "rl", 30, "Scroll length of runlog (set to 0 for no limit)")
	flag.UintVar(&runningJobsLimit, "jl", 8, "Max. concurrently running jobs")
	flag.BoolVar(&attachStdout, "s", false, "set to true to see worker stdout/err if running in terminal")
	flag.BoolVar(&showStagesOnFinished, "F", false, "set to true to show stages on finished jobs in runlog")
	flag.BoolVar(&demoMode, "D", false, "set true/1 to enable public demo mode -- users cannot /shutdown or /rudeshutdown")
	flag.Parse()

	killSwitch = make(chan bool, 1) // ensure a single send can proceed unblocked
	mainCtx := context.Background()

	cmdMap = make(map[string]string)
	runningJobs = make(map[string]*runningJobInfo)

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
	log.Printf("[bacillus %s startup]\n", version)
	log.Printf("[listening on %s]\n", addrPort)

	//log.Printf("Registering handler for /runlog page.\n")
	http.HandleFunc("/runlog", runLogHandler)

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
			launchJobListener(mainCtx, cmd, tag, jobOpts, jobEnv, cmdMap)
		}
	}

	log.Printf("--BACILLUS READY--\n")
	// Seek to end in case we're reusing this runlog to preserve previous
	// entries (yeah it's cheesy and probably error-prone if server was
	// killed during running jobs. Big deal, those entries
	// wouldn't show completion anyhow).
	_, _ = logfile.Seek(0, 2)

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

	//!A http.Handle("/audio/",
	//!A 	http.StripPrefix("/audio/", http.FileServer(http.Dir("audio"))))

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

	// About page
	http.HandleFunc("/about", aboutPageHandler)

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
	<-killSwitch
	_ = server.Shutdown(mainCtx)
}
