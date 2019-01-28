// Helper routines to style the otherwise-plain directory
// listings served out by net/http.FileServer().
//
// Thanks to Tamás Gulácsi for the tip on this
// patch-free method.
//
// Originally I came up with a patch to the actual stdlib
// net/http/fs.go to add hooks one could set to style dirs,
// adding the capability directly to http/fs.go's dirList()
// which is un-exported. That's a bit hacky (and not
// goroutine-friendly, eg. the hooks are global to all
// threads in the program). Tamás' method is much nicer.
//
// One gotcha that took me a while to discover: the HTTP standard
// expects the server to do a local redirect for URIs requesting
// directories without a trailing '/', adding them on. This is
// required for relative links to work from such URIs.
// https://wiki.apache.org/httpd/DirectoryListings
//

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

// Aha! A bug in this, as opposed to the core net/http plain FileServer(),
// where links in a dir listing are wrong if the request does not have
// a trailing '/', is due to expectation that the server will send a
// redirect adding '/'. See
// https://wiki.apache.org/httpd/DirectoryListings
// 'Trailing Slash Redirection'
//
func (fs FileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rootpath, _ := filepath.Abs(strings.TrimPrefix(fs.Root, "/"))
	upath := r.URL.EscapedPath()
	//upath := path.Clean(r.URL.Path)

	var fullpath string
	fullpath = rootpath
	if upath != "." {
		fullpath = fmt.Sprintf("%s%c%s", rootpath, os.PathSeparator, upath)
	}

	if fs, ferr := os.Stat(fullpath); ferr == nil && fs.Mode().IsDir() {
		// IFF upath isn't the root of our virtual FileServer,
		// redirect URIs specifying dirs that don't end in a slash.
		// https://wiki.apache.org/httpd/DirectoryListings
		if upath != "" {
			url := r.URL.Path
			if url[len(url)-1] != '/' {
				localRedirect(w, r, path.Base(url)+"/")
				return
			}
		}
		dirList(w, r, fullpath, upath)
		return
	}
	fs.Handler.ServeHTTP(w, r)
}

// localRedirect gives a Moved Permanently response.
// It does not convert relative paths to absolute paths like Redirect does.
func localRedirect(w http.ResponseWriter, r *http.Request, newPath string) {
	if q := r.URL.RawQuery; q != "" {
		newPath += "?" + q
	}
	w.Header().Set("Location", newPath)
	w.WriteHeader(http.StatusMovedPermanently)
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
		fmt.Fprintf(w, "<a class=\"go-http-fs-item\" href=\"..\">-- up --</a>\n\n")
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
	return "(no files ...)"
}

func usrDirListPost() string {
	return "</pre></body></html>\n"
}
