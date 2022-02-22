package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/brunokim/crdt/diff"
)

var (
	port = flag.Int("port", 8009, "port to run server")
)

func main() {
	flag.Parse()

	http.HandleFunc("/edit", handleEdit)
	http.HandleFunc("/", handleFile)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Serving in %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleFile(w http.ResponseWriter, req *http.Request) {
	path := "." + req.URL.Path
	if path == "./" {
		path = "./static/index.html"
	}
	http.ServeFile(w, req, path)
	log.Printf("%v", path)
}

func handleEdit(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		log.Printf("Error parsing form in /edit: %v\n", err)
		return
	}
	contentT0 := req.Form["contentT0"][0]
	contentT1 := req.Form["contentT1"][0]
	var b strings.Builder
	ops, err := diff.Diff(contentT0, contentT1)
	if err != nil {
		log.Printf("%v", err)
		return
	}
	for _, op := range ops {
		switch op.Op {
		case diff.Keep:
			b.WriteString(" ")
		case diff.Insert:
			b.WriteString("+")
		case diff.Delete:
			b.WriteString("-")
		}
		fmt.Fprintf(&b, "%c", op.Char)
	}
	log.Printf("%s", b.String())
}
