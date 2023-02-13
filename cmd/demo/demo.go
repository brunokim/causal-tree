// This demo simulates several parallel editors in a single web page, forking and syncing their work.
// The state for the web page is kept on this server, where all merging operations are made.
//
// We assume that there is no message loss or out-of-order network shenanigans for this demo.
// An actual, multi-agent edit fest requires a more robust assumption (or, preferrably, that
// the CRDTs are also implemented in the client for powerful syncing).
package main

// Example session:
//  1) User loads demo home webpage (/load)
//  2) Server answers with all current trees, their IDs, contents and connections.
//  3) User edits content for a site (/edit #1)
//  4) User edits content for a site (/edit #2)
//  5) Server answers edit #1, content is compared at that moment in time.
//  6) Server answers edit #2, latest content is compared.
//  7) User forks a site (/fork)
//  8) Server answers with ID and content of new site, as well as everyone's connection.
//  9) User merges two trees (/sync)
// 10) Server responds with new content for merged tree.
//
// Note that connection state is not kept in the server, only on the client.

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
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

type treeinfo struct {
	id    string
	site  *crdt.CausalTree
	mu    *sync.Mutex
	order int
}

func sortTreeinfos(trees []treeinfo) {
	sort.Slice(trees, func(i, j int) bool {
		return trees[i].order < trees[j].order
	})
}

type state struct {
	sync.Mutex

	debugMsgs chan<- debugMessage

	treemap sync.Map // map[string]treeinfo
	maplen  int

	numLoadRequests int
	numEditRequests int
	numForkRequests int
	numSyncRequests int
}

func newState(debugMsgs chan<- debugMessage) *state {
	site := crdt.NewCausalTree()
	siteID := site.SiteID.String()
	tree := treeinfo{
		id:    siteID,
		site:  site,
		mu:    &sync.Mutex{},
		order: 0,
	}
	var treemap sync.Map
	treemap.Store(siteID, tree)
	return &state{
		debugMsgs: debugMsgs,
		treemap:   treemap,
		maplen:    1,
	}
}

func (s *state) treeinfos() []treeinfo {
	var trees []treeinfo
	s.treemap.Range(func(key, val interface{}) bool {
		tree := val.(treeinfo)
		trees = append(trees, tree)
		return true
	})
	sortTreeinfos(trees)
	return trees
}

// -----

func main() {
	flag.Parse()

	debugMsgs := runDebug()
	s := newState(debugMsgs)

	http.Handle("/", http.FileServer(http.Dir(*staticDir)))
	http.Handle("/debug/", http.StripPrefix("/debug", http.FileServer(http.Dir(*debugDir))))
	http.Handle("/load", loadHTTPHandler{s})
	http.Handle("/edit", editHTTPHandler{s})
	http.Handle("/fork", forkHTTPHandler{s})
	http.Handle("/sync", syncHTTPHandler{s})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Serving in %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// -----

type treeResponse struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

type loadResponse struct {
	Trees []treeResponse `json:"trees"`
}

type loadHTTPHandler struct {
	s *state
}

func (h loadHTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.s.handleLoad(w)
}

func (s *state) handleLoad(w http.ResponseWriter) {
	s.writeDebug(map[string]interface{}{
		"Type":    "load",
		"Request": "",
	})
	defer s.syncDebug()
	log.Printf("load")
	//
	s.Lock()
	numRequests := s.numLoadRequests
	s.numLoadRequests++
	s.Unlock()
	// Build response containing all trees.
	var resp loadResponse
	trees := s.treeinfos()
	resp.Trees = make([]treeResponse, len(trees))
	for i, tree := range trees {
		resp.Trees[i] = treeResponse{
			ID:      tree.id,
			Content: tree.site.ToString(),
		}
	}
	bs, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshaling load response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "load error: %v", err)
		return
	}
	// Write response and update internal state.
	w.Header().Set("Content-Type", "application/json")
	w.Write(bs)
	// Write debug info.
	s.writeDebug(map[string]interface{}{
		"Type":    "loadStep",
		"ReqIdx":  numRequests,
		"StepIdx": 0,
		"Sites":   s.debugTrees(),
	})
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
	s.writeDebug(map[string]interface{}{
		"Type":    "edit",
		"Request": req,
	})
	defer s.syncDebug()
	// Retrieve tree from ID and acquire its lock.
	id := req.ID
	val, ok := s.treemap.Load(id)
	if !ok {
		log.Printf("Unknown tree ID: %s", id)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "edit error: %q not found", id)
		return
	}
	tree := val.(treeinfo)
	tree.mu.Lock()
	defer tree.mu.Unlock()
	// Get ID of this edit call.
	s.Lock()
	numRequests := s.numEditRequests
	s.numEditRequests++
	s.Unlock()
	// Execute operations in tree.
	var i int
	for j, op := range req.Ops {
		switch op.Op {
		case "keep":
			i++
		case "insert":
			ch, _ := utf8.DecodeRuneInString(op.Char)
			tree.site.InsertCharAt(ch, i-1)
			log.Printf("%s: operation = insertCharAt %c %d", id, ch, i-1)
			i++
		case "delete":
			tree.site.DeleteAt(i)
			log.Printf("%s: operation = deleteCharAt %d", id, i)
		}
		// Dump trees into debug file.
		if op.Op != "keep" {
			s.writeDebug(map[string]interface{}{
				"Type":     "editStep",
				"ReqIdx":   numRequests,
				"StepIdx":  j,
				"Sites":    s.debugTrees(),
				"LocalIdx": tree.order,
			})
		}
	}
	// Write response with current tree content.
	content := tree.site.ToString()
	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, content)
	log.Printf("%s: value     = %s", id, content)
}

// -----

type forkRequest struct {
	LocalID string `json:"local"`
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
	s.writeDebug(map[string]interface{}{
		"Type":    "fork",
		"Request": req,
	})
	defer s.syncDebug()
	// Retrieve tree from ID and acquire its lock.
	id := req.LocalID
	val, ok := s.treemap.Load(id)
	if !ok {
		log.Printf("Unknown tree ID: %s", id)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "fork error: %q not found", id)
		return
	}
	tree := val.(treeinfo)
	tree.mu.Lock()
	defer tree.mu.Unlock()
	// Get sequence number of this fork call.
	s.Lock()
	order := s.maplen
	numRequests := s.numForkRequests
	s.numForkRequests++
	s.maplen++
	s.Unlock()
	// Fork tree and include it in the treemap.
	remote, err := tree.site.Fork()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "fork error: %v", err)
		return
	}
	remoteID := remote.SiteID.String()
	s.treemap.Store(remoteID, treeinfo{
		id:    remoteID,
		site:  remote,
		mu:    &sync.Mutex{},
		order: order,
	})
	log.Printf("%s: fork      = %s", tree.site.SiteID, remote.SiteID)
	// Write response
	resp := treeResponse{
		ID:      remoteID,
		Content: remote.ToString(),
	}
	bs, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshaling fork response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "fork error: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(bs)
	// Write debug info.
	s.writeDebug(map[string]interface{}{
		"Type":      "forkStep",
		"ReqIdx":    numRequests,
		"StepIdx":   0,
		"Sites":     s.debugTrees(),
		"LocalIdx":  tree.order,
		"RemoteIdx": order,
	})
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
	s.writeDebug(map[string]interface{}{
		"Type":    "sync",
		"Request": req,
	})
	defer s.syncDebug()
	//
	s.Lock()
	numRequests := s.numSyncRequests
	s.numSyncRequests++
	s.Unlock()
	//
	val, ok := s.treemap.Load(req.LocalID)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unknown ID %q", req.LocalID)
		return
	}
	local := val.(treeinfo)
	for i, remoteID := range req.RemoteIDs {
		val, ok := s.treemap.Load(remoteID)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "unknown remote frontend ID: %q", remoteID)
			return
		}
		remote := val.(treeinfo)

		lockAll(local, remote)
		local.site.Merge(remote.site)
		unlockAll(local, remote)

		log.Printf("%s: merge     = %s", req.LocalID, remoteID)
		// Write debug info.
		s.writeDebug(map[string]interface{}{
			"Type":      "syncStep",
			"ReqIdx":    numRequests,
			"StepIdx":   i,
			"Sites":     s.debugTrees(),
			"LocalIdx":  local.order,
			"RemoteIdx": remote.order,
		})
	}
	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, local.site.ToString())
}

// -----

// Lock mutexes in ascending order.
func lockAll(trees ...treeinfo) {
	sortTreeinfos(trees)
	for _, tree := range trees {
		tree.mu.Lock()
	}
}

// Unlock mutexes in descending order.
func unlockAll(trees ...treeinfo) {
	sortTreeinfos(trees)
	for i := len(trees) - 1; i >= 0; i-- {
		trees[i].mu.Unlock()
	}
}

// -----

func (s *state) debugTrees() []*crdt.CausalTree {
	if !s.isDebug() {
		return nil
	}
	treeinfos := s.treeinfos()
	trees := make([]*crdt.CausalTree, len(treeinfos))
	for i, info := range treeinfos {
		trees[i] = info.site
	}
	return trees
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
