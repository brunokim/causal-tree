package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"unicode/utf8"

	"github.com/brunokim/crdt"
)

var (
	port          = flag.Int("port", 8009, "port to run server")
	debug         = flag.Bool("debug", false, "whether to dump debug information. Default debug file is log_{{datetime}}.jsonl")
	debugFilename = flag.String("debug_file", "", "file to dump debug information in JSONL format. Implies --debug")

	listmap         = map[string]*crdt.RList{}
	listFrontendIDs = []string{}

	debugFile *os.File

	numEditRequests int
)

func main() {
	flag.Parse()

	if *debug && *debugFilename == "" {
		datetime := time.Now().Format("2006-01-02T15:04:05")
		*debugFilename = fmt.Sprintf("log_%s.jsonl", datetime)
	}
	if *debugFilename != "" {
		var err error
		debugFile, err = os.Create(*debugFilename)
		if err != nil {
			log.Printf("Error opening debug file: %v", err)
		}
	}

	http.Handle("/debug/", http.StripPrefix("/debug", http.FileServer(http.Dir("../debug"))))
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

type editRequest struct {
	ID  string          `json:"id"`
	Ops []editOperation `json:"ops"`
}

type editOperation struct {
	Op   string `json:"op"`
	Char string `json:"ch"`
	Dist int    `json:"dist"`
}

// TODO: synchronize access to shared state.
func handleEdit(w http.ResponseWriter, req *http.Request) {
	parser := json.NewDecoder(req.Body)
	editReq := &editRequest{}
	if err := parser.Decode(editReq); err != nil {
		log.Printf("Error parsing body in /edit: %v", err)
		return
	}
	writeDebug(editReq)
	id := editReq.ID
	if _, ok := listmap[id]; !ok {
		listmap[id] = crdt.NewRList()
		listFrontendIDs = append(listFrontendIDs, id)
	}
	// Execute operations in list.
	var i int
	for j, op := range editReq.Ops {
		switch op.Op {
		case "keep":
			i++
		case "insert":
			ch, _ := utf8.DecodeRuneInString(op.Char)
			listmap[id].InsertCharAt(ch, i-1)
			i++
		case "delete":
			listmap[id].DeleteCharAt(i)
		}
		// Dump lists into debug file.
		if op.Op != "keep" && debugFile != (*os.File)(nil) {
			lists := make([]*crdt.RList, len(listmap))
			for i, id := range listFrontendIDs {
				lists[i] = listmap[id]
			}
			writeDebug(map[string]interface{}{
				"ReqIdx": numEditRequests,
				"OpIdx":  j,
				"Sites":  lists,
			})
		}
	}
	syncDebug()
	numEditRequests++
	log.Printf("%s: %s", id, listmap[id].AsString())
}

// TODO: write and sync debug info in another goroutine
func writeDebug(x interface{}) {
	if debugFile == (*os.File)(nil) {
		return
	}
	bs, err := json.Marshal(x)
	if err != nil {
		log.Printf("Error while writing to debug file: %v", err)
		debugFile.Close()
		debugFile = nil
		return
	}
	debugFile.Write(bs)
	debugFile.WriteString("\n")
}

func syncDebug() {
	if debugFile != (*os.File)(nil) {
		debugFile.Sync()
	}
}
