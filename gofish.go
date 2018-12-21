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
	"strings"
)

const (
	appVer string = "v0.1"
	defKey string = "IAmABanana"
)

var (
	execRoot string
	addrPort string
	hookStd  string
	apiKey   string
)

// TODO: types for matching JSON events of
// supported webhooks: gogs.io, github, gitlab, ... ?

func main() {
	flag.StringVar(&execRoot, "r", "", "root path prefix for endpoint cmds")
	flag.StringVar(&addrPort, "a", ":9990", "[addr]:port on which to listen")
	flag.StringVar(&hookStd, "t", "blind", "hook type")
	flag.StringVar(&apiKey, "k", defKey, "API key")
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
		fmt.Printf("Registering handler for %s/%s [action %s]...\n", hookStd, tag, cmd)
		http.HandleFunc(fmt.Sprintf("/%s/%s", hookStd, tag),
			func(w http.ResponseWriter, r *http.Request) {
				log.Printf("Ooh a shiny hook that says '%s'! Let's bite...\n", r.URL)
				if cmdMap[tag][0] != '/' {
					log.Printf("Activate command: [%s]\n", execRoot+"/"+cmdMap[tag])
				} else {
					log.Printf("Activate command: [%s]\n", cmdMap[tag])
				}
				// ...
				w.Write([]byte("OK"))
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
