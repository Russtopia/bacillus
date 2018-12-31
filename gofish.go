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
	//execRoot     string
	addrPort     string
	hookStd      string
	apiKey       string
	attachStdout bool
)

// TODO: types for matching JSON events of
// supported webhooks: gogs.io, github, gitlab, ... ?

func main() {
	//flag.StringVar(&execRoot, "r", "/tmp", "root path prefix for endpoint cmds")
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

	for _, e := range flag.Args() {
		fields := strings.Split(e, ":")
		tag := fields[0]
		var jobHomeDir string
		var jobEnv []string
		var cmd string
		if len(fields) > 1 { /*&& fields[1] != "" {*/
			jobHomeDir = fields[1]
		}
		if len(fields) > 2 { /*&& fields[2] != "" {*/
			jobEnv = strings.Split(fields[2], ",")
		}
		if len(fields) > 3 && fields[3] != "" {
			cmd = fields[3]
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

					if jobHomeDir == "" {
						// if jobHomeDir is not specified, default will be
						// main program's and always remove GOFISH_WORKDIR
						// after the run (since this will imply use of /tmp
						// and WORKDIR should not persist there)
						c.Env = append(c.Env,
							fmt.Sprintf("GOFISH_REMOVE_WORKDIR=1"))
					} else {
						// if jobHomeDir is set, set execution dir to
						// it and let user define whether or not to persist
						// WORKDIR via the GOFISH_WORKDIR env var.
						jobHomeDir, _ = filepath.Abs(jobHomeDir)
						c.Dir = jobHomeDir
					}

					//var terr error
					//var workDir string
					workDir, terr := ioutil.TempDir(jobHomeDir, "gofish_")
					jobID := strings.Split(workDir, "_")[1]
					indent, _ := strconv.ParseInt(jobID, 10, 64)
					indentStr := strings.Repeat("-", int(indent%8))
					log.Printf("%s[webhook %s{%s}: %s]\n", indentStr,
						tag, jobID, cmdMap[tag])
					if terr != nil {
						log.Printf("[ERROR creating workdir (%s) for event %s trigger.]\n", terr, tag)
					} else {
						if attachStdout {
							c.Stdout = os.Stdout
							c.Stderr = os.Stderr
						} else {
							workerOutputFile, _ := os.Create(workDir + "/console.out")
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

	err := http.ListenAndServe(addrPort, nil)
	if err != nil {
		log.Fatal(err)
	}
}

//func hookHandler(w http.ResponseWriter, r *http.Request) {
//	log.Println("Ooh a shiny hook! Let's bite...")
//	// ...
//	w.Write([]byte("OK"))
//}
