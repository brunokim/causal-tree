package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/brunokim/crdt"
	"github.com/brunokim/crdt/diff"
)

var (
	port          = flag.Int("port", 8009, "port to run server")
	debugFilename = flag.String("debug_file", "", "file to dump debug information in JSONL format")

	lists = map[string]*crdt.RList{}

	debugFile *os.File
)

func main() {
	flag.Parse()

	if *debugFilename != "" {
		var err error
		debugFile, err = os.Create(*debugFilename)
		if err != nil {
			log.Printf("Error opening debug file: %v", err)
		}
	}

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
	id := req.Form["id"][0]
	if _, ok := lists[id]; !ok {
		lists[id] = crdt.NewRList()
	}
	contentT0 := req.Form["contentT0"][0]
	contentT1 := req.Form["contentT1"][0]
	ops, err := diff.Diff(contentT0, contentT1)
	if err != nil {
		log.Printf("%v", err)
		return
	}
	// Execute operations in list.
	var i int
	for _, op := range ops {
		switch op.Op {
		case diff.Keep:
			i++
		case diff.Insert:
			lists[id].InsertCharAt(op.Char, i-1)
			i++
		case diff.Delete:
			lists[id].DeleteCharAt(i)
		}
	}
	log.Printf("%s: %s", id, lists[id].AsString())
	if debugFile != (*os.File)(nil) {
		// Dump lists into debug file.
		bs, err := json.Marshal(map[string]interface{}{
			"Params": req.Form,
			"Sites":  lists,
		})
		if err != nil {
			log.Printf("Error while writing to debug file: %v", err)
			debugFile.Close()
			debugFile = nil
		} else {
			debugFile.Write(bs)
			debugFile.WriteString("\n")
		}
	}
}
