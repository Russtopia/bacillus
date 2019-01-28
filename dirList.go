/* -*- go -*-
 *  $RCSfile$ $Revision$ : $Date$ : $Author$
 *
 *  Description
 *
 *  Notes
 *
 **************
 *
 *  Copyright (c) 2019 Russtopia Labs. All Rights Reserved.
 *
 * This document may not, in whole or in part, be copied, photocopied,
 * reproduced, translated, or reduced to any electronic medium or machine
 * readable form without prior written consent from Russtopia Labs.
 */
package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type FileServer struct {
	Root string
	http.Handler
}

func (fs FileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upath := path.Clean(r.URL.Path)
	fmt.Println("upath:", upath)
	abspath, aerr := filepath.Abs(filepath.Join(strings.TrimPrefix(fs.Root, "/")))
	if upath != "." {
		abspath = fmt.Sprintf("%s%c%s", abspath, os.PathSeparator, upath)
		fmt.Println("abspath:", abspath)
	}
	if aerr == nil {
		if fi, err := os.Stat(abspath); err == nil && fi.Mode().IsDir() {
			dirList(w, r, abspath, upath)
			return
		}
	} else {
		log.Println(aerr)
	}
	fs.Handler.ServeHTTP(w, r)
}

var htmlReplacer = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	// "&#34;" is shorter than "&quot;".
	`"`, "&#34;",
	// "&#39;" is shorter than "&apos;" and apos was not in HTML until HTML5.
	"'", "&#39;",
)

func dirList(w http.ResponseWriter, r *http.Request, dir string, upath string) {
	//fmt.Println("dir:", dir)
	f, err := os.Open(dir)
	if err != nil {
		log.Printf("http: error reading directory: %v", err)
		http.Error(w, "Error reading directory", http.StatusInternalServerError)
		return
	}
	dirs, err := f.Readdir(-1)
	if err != nil {
		log.Printf("http: error reading directory: %v", err)
		http.Error(w, "Error reading directory", http.StatusInternalServerError)
		return
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })

	var headers map[string]string
	var preamble string
	headers, preamble = usrDirListPre(r)

	for h, v := range headers {
		w.Header().Set(h, v)
	}

	fmt.Fprintf(w, preamble)

	if upath != "." {
		fmt.Fprintf(w, "<a class=\"go-http-fs-item\" href=\"..\">-- up --</a>\n")
	}
	if len(dirs) == 0 {
		fmt.Fprintf(w, usrDirListE())
	} else {
		for _, d := range dirs {
			name := d.Name()
			if d.IsDir() {
				name += "/"
			}
			// name may contain '?' or '#', which must be escaped to remain
			// part of the URL path, and not indicate the start of a query
			// string or fragment.
			url := url.URL{Path: name}
			fmt.Printf("url: %q\n", url)
			fmt.Println("url.String():", url.String())
			fmt.Fprintf(w, "<a class=\"go-http-fs-item\" href=\"%s\">%s</a>\n", url.String(), htmlReplacer.Replace(name))
		}
	}
	fmt.Fprintf(w, usrDirListPost())
}

func usrDirListPre(r *http.Request) (hdrs map[string]string, preamble string) {
	hdrs = make(map[string]string)
	hdrs["Content-Type"] = "text/html; charset=utf-8"
	//hdrs["X-Foo"] = "bacillus dir listing"
	preamble = `
	<head>` +
		getFavIcon() + `
	</head>
	<body ` + getBodyBgndHTMLFrag() + `>
	<img style="float:left;" width="16px" src="/images/logo.jpg"/><pre style='background-color: grey;'><a class="go-http-fs-home" href="/">bacill&mu;s ` + appVer + `</a> ---- directory: ` + fmt.Sprintf(r.URL.Path) + ` ----</pre>
	<pre>`
	return
}

func usrDirListE() string {
	return "\n(no files ...)"
}

func usrDirListPost() string {
	return "</pre></body></html>\n"
}
