package main

import (
	"encoding/json"
	"fmt"
	mux "github.com/gorilla/mux"
	p2p_peer "github.com/ipfs/go-libp2p-peer"
	mc "github.com/mediachain/concat/mc"
	mcq "github.com/mediachain/concat/mc/query"
	pb "github.com/mediachain/concat/proto"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

func (node *Node) httpId(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, node.Identity.Pretty())
}

func (node *Node) httpPing(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	peerId := vars["peerId"]
	pid, err := p2p_peer.IDB58Decode(peerId)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error: Bad id: %s\n", err.Error())
		return
	}

	err = node.doPing(r.Context(), pid)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Error: %s\n", err.Error())
		return
	}

	fmt.Fprintln(w, "OK")
}

func (node *Node) httpPublish(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ns := vars["namespace"]

	rbody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("http/publish: Error reading request body: %s", err.Error())
		return
	}

	// just simple statements for now
	sbody := new(pb.SimpleStatement)
	err = json.Unmarshal(rbody, sbody)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error: %s\n", err.Error())
		return
	}

	sid, err := node.doPublish(ns, sbody)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error: %s\n", err.Error())
		return
	}

	fmt.Fprintln(w, sid)
}

func (node *Node) httpStatement(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["statementId"]

	stmt, err := node.db.Get(id)
	if err != nil {
		switch err {
		case UnknownStatement:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "Unknown statement\n")
			return

		default:
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error: %s\n", err.Error())
			return
		}
	}

	err = json.NewEncoder(w).Encode(stmt)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error: %s\n", err.Error())
		return
	}
}

func (node *Node) httpQuery(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("http/query: Error reading request body: %s", err.Error())
		return
	}

	q, err := mcq.ParseQuery(string(body))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error: %s\n", err.Error())
		return
	}

	res, err := node.db.Query(q)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error: %s\n", err.Error())
		return
	}

	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error: %s\n", err.Error())
		return
	}
}

func (node *Node) httpStatus(w http.ResponseWriter, r *http.Request) {
	status := statusString[node.status]
	fmt.Fprintln(w, status)
}

func (node *Node) httpStatusSet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	state := vars["state"]

	var err error
	switch state {
	case "offline":
		err = node.goOffline()

	case "online":
		err = node.goOnline()

	case "public":
		err = node.goPublic()

	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Bad state: %s\n", state)
		return
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error: %s\n", err.Error())
		return
	}

	fmt.Fprintln(w, statusString[node.status])
}

func (node *Node) httpConfigDir(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodHead:
		return
	case http.MethodGet:
		if node.dir != nil {
			fmt.Fprintln(w, mc.FormatHandle(*node.dir))
		} else {
			fmt.Fprintln(w, "nil")
		}
	case http.MethodPost:
		node.httpConfigDirSet(w, r)

	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Unsupported method: %s", r.Method)
	}
}

func (node *Node) httpConfigDirSet(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("http/query: Error reading request body: %s", err.Error())
		return
	}

	handle := strings.TrimSpace(string(body))
	pinfo, err := mc.ParseHandle(handle)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error: %s\n", err.Error())
		return
	}

	node.dir = &pinfo
	fmt.Fprintln(w, "OK")
}
