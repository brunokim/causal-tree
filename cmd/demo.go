package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/brunokim/crdt"
)

var (
	port          = flag.Int("port", 8009, "port to run server")
	debug         = flag.Bool("debug", false, "whether to dump debug information. Default debug file is log_{{datetime}}.jsonl")
	debugFilename = flag.String("debug_file", "", "file to dump debug information in JSONL format. Implies --debug")
)

type debugMsgType int

const (
	writeDebug debugMsgType = iota
	syncDebug
)

type debugMessage struct {
	msgType debugMsgType
	payload interface{}
}

type state struct {
	sync.Mutex

	debugMsgs chan<- debugMessage

	listmap         map[string]*crdt.RList
	listFrontendIDs []string

	numEditRequests int
}

func newState(debugMsgs chan<- debugMessage) *state {
	return &state{
		debugMsgs: debugMsgs,
		listmap:   make(map[string]*crdt.RList),
	}
}

// -----

func main() {
	flag.Parse()

	debugMsgs := runDebug()
	s := newState(debugMsgs)

	http.Handle("/debug/", http.StripPrefix("/debug", http.FileServer(http.Dir("../debug"))))
	http.Handle("/edit", editHTTPHandler{s})
	http.Handle("/fork", forkHTTPHandler{s})
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

// -----

type editMessage struct {
	w   http.ResponseWriter
	req *editRequest
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

type editHTTPHandler struct {
	s *state
}

func (h editHTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	parser := json.NewDecoder(req.Body)
	editReq := &editRequest{}
	if err := parser.Decode(editReq); err != nil {
		log.Printf("Error parsing body in /edit: %v", err)
		return
	}
	h.s.handleEdit(w, editReq)
}

func (s *state) handleEdit(w http.ResponseWriter, req *editRequest) {
	s.Lock()
	defer s.Unlock()
	s.writeDebug(req)

	id := req.ID
	if _, ok := s.listmap[id]; !ok {
		s.listmap[id] = crdt.NewRList()
		s.listFrontendIDs = append(s.listFrontendIDs, id)
	}
	// Execute operations in list.
	var i int
	for j, op := range req.Ops {
		switch op.Op {
		case "keep":
			i++
		case "insert":
			ch, _ := utf8.DecodeRuneInString(op.Char)
			s.listmap[id].InsertCharAt(ch, i-1)
			log.Printf("%s: operation = insertCharAt %c %d", id, ch, i-1)
			i++
		case "delete":
			s.listmap[id].DeleteCharAt(i)
			log.Printf("%s: operation = deleteCharAt %d", id, i)
		}
		// Dump lists into debug file.
		if op.Op != "keep" && s.isDebug() {
			lists := make([]*crdt.RList, len(s.listmap))
			for i, id := range s.listFrontendIDs {
				lists[i] = s.listmap[id]
			}
			s.writeDebug(map[string]interface{}{
				"ReqIdx": s.numEditRequests,
				"OpIdx":  j,
				"Sites":  lists,
			})
		}
	}
	content := s.listmap[id].AsString()
	log.Printf("%s: value     = %s", id, content)

	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, content)

	s.syncDebug()
	s.numEditRequests++
}

// -----

type forkRequest struct {
	LocalID  string `json:"local"`
	RemoteID string `json:"remote"`
}

type forkHTTPHandler struct {
	s *state
}

func (h forkHTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	parser := json.NewDecoder(req.Body)
	forkReq := &forkRequest{}
	if err := parser.Decode(forkReq); err != nil {
		log.Printf("Error parsing body in /fork: %v", err)
		return
	}
	h.s.handleFork(w, forkReq)
}

func (s *state) handleFork(w http.ResponseWriter, req *forkRequest) {
	s.Lock()
	defer s.Unlock()
	s.writeDebug(req)

	if _, ok := s.listmap[req.LocalID]; !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unknown local frontend ID %q", req.LocalID)
		return
	}
	if _, ok := s.listmap[req.RemoteID]; ok {
		w.WriteHeader(http.StatusPreconditionFailed)
		fmt.Fprintf(w, "new remote frontend ID already exists: %q", req.RemoteID)
		return
	}
	s.listmap[req.RemoteID] = s.listmap[req.LocalID].Fork()
	log.Printf("%s: fork      = %s", req.LocalID, req.RemoteID)

	s.syncDebug()
}

// -----

func (s *state) isDebug() bool {
	return s.debugMsgs != nil
}

func (s *state) writeDebug(x interface{}) {
	if s.isDebug() {
		s.debugMsgs <- debugMessage{
			msgType: writeDebug,
			payload: x,
		}
	}
}

func (s *state) syncDebug() {
	if s.isDebug() {
		s.debugMsgs <- debugMessage{msgType: syncDebug}
	}
}

func runDebug() chan<- debugMessage {
	f := createDebug()
	if f == nil {
		return nil
	}
	ch := make(chan debugMessage, 10)
	go func() {
		for msg := range ch {
			if f == nil {
				continue
			}
			switch msg.msgType {
			case writeDebug:
				if bs, err := json.Marshal(msg.payload); err != nil {
					log.Printf("Error while writing to debug file: %v", err)
				} else {
					f.Write(bs)
					f.WriteString("\n")
				}
			case syncDebug:
				f.Sync()
			}
		}
		f.Close()
	}()
	return ch
}

func createDebug() *os.File {
	if !*debug && *debugFilename == "" {
		return nil
	}
	if *debugFilename == "" {
		datetime := time.Now().Format("2006-01-02T15:04:05")
		*debugFilename = fmt.Sprintf("log_%s.jsonl", datetime)
	}
	debugFile, err := os.Create(*debugFilename)
	if err != nil {
		log.Printf("Error opening debug file: %v", err)
		return nil
	}
	return debugFile
}
