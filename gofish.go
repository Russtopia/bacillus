// Package gofish is a Webhook listener that dispatches
// arbitrary commands on receipt of webhook events.
// Supported webhook event formats: gogs, (.. future)
//
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

const (
	appVer string = "v0.1"
	defKey string = "IAmABanana"
)

var (
	execRoot     string
	addrPort     string
	hookStd      string
	apiKey       string
	attachStdout bool
)

// TODO: types for matching JSON events of
// supported webhooks: gogs.io, github, gitlab, ... ?

func main() {
	flag.StringVar(&execRoot, "r", "/tmp", "root path prefix for endpoint cmds")
	flag.StringVar(&addrPort, "a", ":9990", "[addr]:port on which to listen")
	flag.StringVar(&hookStd, "t", "blind", "hook type")
	flag.StringVar(&apiKey, "k", defKey, "API key")
	flag.BoolVar(&attachStdout, "s", false, "set to true to see worker stdout/err if running in terminal")
	flag.Parse()
	log.Printf("gofish %s startup\n", appVer)
	log.Printf("listening on %s, type %s\n", addrPort, hookStd)

	cmdMap := make(map[string]string)

	for _, e := range flag.Args() {
		fields := strings.Split(e, ":")
		tag := fields[0]
		cmd := "null"
		if len(fields) > 1 && fields[1] != "" {
			cmd = fields[1]
		}

		cmdMap[tag] = cmd
		log.Printf("Registering handler for %s/%s [action %s]...\n", hookStd, tag, cmd)
		http.HandleFunc(fmt.Sprintf("/%s/%s", hookStd, tag),
			func(w http.ResponseWriter, r *http.Request) {
				cmdStrList := strings.Split(cmdMap[tag], " ")
				cmdArgs := []string{""}
				if len(cmdStrList) > 1 {
					cmdArgs = cmdStrList[1:]
				}
				log.Printf("webhook '%s': %s\n", tag, cmdMap[tag])
				c := exec.Command(cmdStrList[0], cmdArgs...)
				c.Dir = execRoot
				if attachStdout {
					c.Stdout = os.Stdout
					c.Stderr = os.Stderr
				}
				cerr := c.Run()
				if cerr != nil {
					log.Printf("[exec.Cmd: %+v]\n", c)
				} else {
					w.Write([]byte("OK"))
					log.Println("[done]")
				}
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
