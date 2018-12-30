// Package gofish is a Webhook listener that dispatches
// arbitrary commands on receipt of webhook events.
// Supported webhook event formats: gogs, (.. future)
//
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
		log.Println("TODO: /runlog page")
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
					log.Printf("[webhook '%s': %s]\n", tag, cmdMap[tag])
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
						c.Env = append(c.Env, fmt.Sprintf("GOFISH_JOBTAG=%s", tag))
						c.Env = append(c.Env, fmt.Sprintf("GOFISH_WORKDIR=%s", workDir))
						c.Env = append(c.Env, jobEnv...)

						cerr := c.Start()
						if cerr != nil {
							log.Printf("[exec.Cmd: %+v]\n", c)
							w.WriteHeader(500)
							w.Write([]byte("ERR"))
							log.Printf("[ERROR on event %s trigger.]\n", tag)
						} else {
							w.Write([]byte("OK"))
							log.Printf("[event %s triggered. (workDir %s)]\n", tag, workDir)
						}
						werr := c.Wait()
						if werr == nil {
							log.Printf("[event %s completed with status 0]\n", tag)
						} else {
							log.Printf("[event %s completed with error %s]\n", tag, werr)
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
