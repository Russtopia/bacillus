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

// FileServer represents a served filesystem
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
	if !httpAuthSession(w, r) {
		return
	}

	sortOrder := "newest" // name || newest || oldest
	v, ok := r.URL.Query()["sort"]
	if ok {
		fmt.Sscanf(v[0], "%s", &sortOrder) // nolint:errcheck
	}

	rootpath, _ := filepath.Abs(strings.TrimPrefix(fs.Root, "/"))
	upath := r.URL.EscapedPath()
	//upath := path.Clean(r.URL.Path)

	fullpath := rootpath
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
		dirList(w, r, fullpath, upath, sortOrder)
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

func itemCountStr(l int) (s string) {
	if l == 1 {
		s = "1 item"
	} else {
		s = fmt.Sprintf("%d items", l)
	}
	return
}

func dirList(w http.ResponseWriter, r *http.Request, dir, upath, sortOrder string) {
	f, err := os.Open(dir)
	if err != nil {
		log.Printf("http: error reading directory: %v", err)
		http.Error(w, "Error reading directory", http.StatusInternalServerError)
		return
	}
	items, err := f.Readdir(-1)
	if err != nil {
		log.Printf("http: error reading directory: %v", err)
		http.Error(w, "Error reading directory", http.StatusInternalServerError)
		return
	}
	var sortFunc func(i, j int) bool
	switch sortOrder {
	case "name":
		sortFunc = func(i, j int) bool { return items[i].Name() < items[j].Name() }
	case "oldest":
		sortFunc = func(i, j int) bool {
			return items[i].ModTime().Before(items[j].ModTime())
		}
	case "newest":
		fallthrough
	default:
		sortFunc = func(i, j int) bool {
			return items[i].ModTime().After(items[j].ModTime())
		}
	}

	sort.Slice(items, sortFunc)

	var headers map[string]string
	var preamble string
	headers, preamble = usrDirListPre(r)

	for h, v := range headers {
		w.Header().Set(h, v)
	}

	_, _ = fmt.Fprintf(w, preamble)

	if upath != "." {
		_, _ = fmt.Fprintf(w,
			"<a class=\"go-http-fs-item\" href=\"..\">-- up --</a>\n\n")
	}
	if len(items) == 0 {
		_, _ = fmt.Fprintf(w, usrDirListE())
	} else {
		_, _ = fmt.Fprint(w, itemCountStr(len(items))+"\n")
		for _, d := range items {
			name := d.Name()
			if d.IsDir() {
				name += "/"
			}
			// name may contain '?' or '#', which must be escaped to remain
			// part of the URL path, and not indicate the start of a query
			// string or fragment.
			url := url.URL{Path: name}
			_, _ = fmt.Fprintf(w,
				"<a class=\"go-http-fs-item\" href=\"%s\">%-48s</a>%-16s%30s\n",
				url.String(),
				htmlReplacer.Replace(name),
				" ",
				d.ModTime().Format("Mon Jan 2 15:04:05 MST 2006"))
		}
	}
	_, _ = fmt.Fprintf(w, usrDirListPost())
}

func dirLinkStyle() string {
	return `
	<style>
	a.go-http-fs-item {
			display: inline-block;
			text-decoration: none;
			color: inherit;
		}
		a.go-http-fs-item:visited {
			color: inherit;
		}
		a.go-http-fs-item:hover {
			background-color: aliceblue;
			text-decoration: underline;
			text-decoration-style: dotted;
			cursor: pointer;
		}
		a.go-http-fs-item:active {
			background-color: lightgreen;
		}
	</style>`
}

func usrDirListPre(r *http.Request) (hdrs map[string]string, preamble string) {
	hdrs = make(map[string]string)
	hdrs["Content-Type"] = "text/html; charset=utf-8"
	//hdrs["X-Foo"] = "bacillus dir listing"
	preamble = `
	<head>` +
		favIconHTML() +
		dirLinkStyle() + `
	</head>
	<body ` + bodyBgndHTMLAttribs() + `>
	<img style="float:left;" width="16px" src="/images/logo.jpg"/><pre style='background-color: grey;'><a class="go-http-fs-home" href="/">bacill&mu;s ` + version + `</a> ---- directory: ` + fmt.Sprintf(r.URL.Path) + ` ----</pre>
	<pre>`
	return
}

func usrDirListE() string {
	return "(no files ...)"
}

func usrDirListPost() string {
	return "</pre></body></html>\n"
}
