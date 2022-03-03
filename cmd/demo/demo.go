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

	"github.com/brunokim/causal-tree/crdt"
)

var (
	port          = flag.Int("port", 8009, "port to run server")
	debug         = flag.Bool("debug", false, "whether to dump debug information. Default debug file is log_{{datetime}}.jsonl")
	debugFilename = flag.String("debug_file", "", "file to dump debug information in JSONL format. Implies --debug")

	staticDir = flag.String("static_dir", "", "Directory with static files")
	debugDir  = flag.String("debug_dir", "", "Directory with static debug files")
)

// -----

type debugMsgType int

const (
	writeDebug debugMsgType = iota
	syncDebug
)

type debugMessage struct {
	msgType debugMsgType
	payload interface{}
}

// -----

type state struct {
	sync.Mutex

	debugMsgs chan<- debugMessage

	listmap         map[string]*crdt.RList
	listFrontendIDs []string

	numEditRequests int
	numForkRequests int
	numSyncRequests int
}

func newState(debugMsgs chan<- debugMessage) *state {
	return &state{
		debugMsgs: debugMsgs,
		listmap:   make(map[string]*crdt.RList),
	}
}

func index(y string, xs []string) int {
	for i, x := range xs {
		if x == y {
			return i
		}
	}
	return len(xs)
}

// -----

func main() {
	flag.Parse()

	debugMsgs := runDebug()
	s := newState(debugMsgs)

	http.Handle("/", http.FileServer(http.Dir(*staticDir)))
	http.Handle("/debug/", http.StripPrefix("/debug", http.FileServer(http.Dir(*debugDir))))
	http.Handle("/edit", editHTTPHandler{s})
	http.Handle("/fork", forkHTTPHandler{s})
	http.Handle("/sync", syncHTTPHandler{s})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Serving in %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// -----

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
	s.writeDebug(map[string]interface{}{
		"Type":    "edit",
		"Request": req,
	})

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
		if op.Op != "keep" {
			s.writeDebug(map[string]interface{}{
				"Type":     "editStep",
				"ReqIdx":   s.numEditRequests,
				"StepIdx":  j,
				"Sites":    s.debugLists(),
				"LocalIdx": index(id, s.listFrontendIDs),
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
	s.writeDebug(map[string]interface{}{
		"Type":    "fork",
		"Request": req,
	})

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
	s.listFrontendIDs = append(s.listFrontendIDs, req.RemoteID)
	log.Printf("%s: fork      = %s", req.LocalID, req.RemoteID)

	s.writeDebug(map[string]interface{}{
		"Type":      "forkStep",
		"ReqIdx":    s.numForkRequests,
		"StepIdx":   0,
		"Sites":     s.debugLists(),
		"LocalIdx":  index(req.LocalID, s.listFrontendIDs),
		"RemoteIdx": index(req.RemoteID, s.listFrontendIDs),
	})
	s.numForkRequests++
	s.syncDebug()
}

// -----

type syncRequest struct {
	LocalID   string   `json:"id"`
	RemoteIDs []string `json:"mergeIds"`
}

type syncHTTPHandler struct {
	s *state
}

func (h syncHTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	parser := json.NewDecoder(req.Body)
	syncReq := &syncRequest{}
	if err := parser.Decode(syncReq); err != nil {
		log.Printf("Error parsing body in /sync: %v", err)
		return
	}
	h.s.handleSync(w, syncReq)
}

func (s *state) handleSync(w http.ResponseWriter, req *syncRequest) {
	s.Lock()
	defer s.Unlock()
	s.writeDebug(map[string]interface{}{
		"Type":    "sync",
		"Request": req,
	})

	local, ok := s.listmap[req.LocalID]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unknown local frontend ID %q", req.LocalID)
		return
	}
	for i, remoteID := range req.RemoteIDs {
		remote, ok := s.listmap[remoteID]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "unknown remote frontend ID: %q", remoteID)
			return
		}
		local.Merge(remote)
		log.Printf("%s: merge     = %s", req.LocalID, remoteID)

		s.writeDebug(map[string]interface{}{
			"Type":      "syncStep",
			"ReqIdx":    s.numSyncRequests,
			"StepIdx":   i,
			"Sites":     s.debugLists(),
			"LocalIdx":  index(req.LocalID, s.listFrontendIDs),
			"RemoteIdx": index(remoteID, s.listFrontendIDs),
		})
	}

	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, local.AsString())

	s.syncDebug()
	s.numSyncRequests++
}

// -----

func (s *state) debugLists() []*crdt.RList {
	if !s.isDebug() {
		return nil
	}
	lists := make([]*crdt.RList, len(s.listmap))
	for i, id := range s.listFrontendIDs {
		lists[i] = s.listmap[id]
	}
	return lists
}

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
