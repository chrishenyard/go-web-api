package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"slices"

	"github.com/chrishenyard/go-standard-library/graphs"
)

const (
	maxWeight   = 1_000_000
	maxBodySize = 1 << 16
)

var nodeIDs = []string{"A", "B", "C", "D", "E", "F", "G", "H"}

type edgeDef struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Weight int    `json:"weight"`
}

var defaultEdges = []edgeDef{
	{"A", "B", 4}, {"A", "C", 2}, {"B", "D", 5}, {"B", "E", 3},
	{"C", "B", 1}, {"C", "E", 6}, {"C", "F", 8}, {"D", "G", 3},
	{"E", "D", 1}, {"E", "F", 2}, {"E", "G", 6}, {"F", "H", 4},
	{"G", "H", 2}, {"H", "A", 9}, {"F", "C", 3}, {"G", "E", 2},
}

var networkPositions = map[string][2]float64{
	"A": {85, 270}, "B": {265, 130}, "C": {265, 410}, "D": {455, 80},
	"E": {455, 265}, "F": {455, 465}, "G": {650, 175}, "H": {745, 370},
}

type graphData struct {
	Nodes     []string              `json:"nodes"`
	Edges     []edgeDef             `json:"edges"`
	Positions map[string][2]float64 `json:"positions"`
	MaxWeight int                   `json:"maxWeight"`
}

type pageView struct {
	GraphJSON template.JS
}

type solveRequest struct {
	Start   string `json:"start"`
	Weights []int  `json:"weights"`
}

type nodeResult struct {
	Reachable bool     `json:"reachable"`
	Cost      int      `json:"cost"`
	Path      []string `json:"path"`
}

type solveResponse struct {
	Results map[string]nodeResult `json:"results"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type dijkstraHandler struct {
	tmpl      *template.Template
	graphJSON template.JS
}

func newDijkstraHandler(templatesDir string) (*dijkstraHandler, error) {
	t, err := template.ParseFiles(templatesDir + "/index.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	// json.Marshal escapes <, > and &, so the output is safe inside a script tag.
	b, err := json.Marshal(graphData{
		Nodes: nodeIDs, Edges: defaultEdges,
		Positions: networkPositions, MaxWeight: maxWeight,
	})
	if err != nil {
		return nil, fmt.Errorf("encode graph: %w", err)
	}
	return &dijkstraHandler{tmpl: t, graphJSON: template.JS(b)}, nil
}

// ServeHTTP serves the page shell on GET and the shortest-path computation on POST.
func (h *dijkstraHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		h.servePage(w, r)
	case http.MethodPost:
		h.serveSolve(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *dijkstraHandler) servePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.Execute(w, pageView{GraphJSON: h.graphJSON}); err != nil {
		slog.ErrorContext(r.Context(), "render dijkstra page", "error", err)
	}
}

func (h *dijkstraHandler) serveSolve(w http.ResponseWriter, r *http.Request) {
	var req solveRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodySize))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{"Invalid request body."})
		return
	}
	if !slices.Contains(nodeIDs, req.Start) {
		writeJSON(w, http.StatusBadRequest, errorResponse{"Unknown starting node."})
		return
	}
	if len(req.Weights) != len(defaultEdges) {
		writeJSON(w, http.StatusBadRequest, errorResponse{"Wrong number of edge costs."})
		return
	}
	for _, n := range req.Weights {
		if n < 0 || n > maxWeight {
			writeJSON(w, http.StatusBadRequest, errorResponse{"Enter a whole-number cost from 0 to 1,000,000."})
			return
		}
	}

	g := graphs.New[string]()
	for _, id := range nodeIDs {
		g.AddNode(id)
	}
	for i, e := range defaultEdges {
		if err := g.AddEdge(e.From, e.To, req.Weights[i]); err != nil {
			slog.ErrorContext(r.Context(), "add edge", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{"Could not build graph."})
			return
		}
	}
	results, err := g.ShortestPaths(req.Start)
	if err != nil {
		slog.ErrorContext(r.Context(), "shortest paths", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{"Could not compute shortest paths."})
		return
	}

	resp := solveResponse{Results: make(map[string]nodeResult, len(nodeIDs))}
	for _, id := range nodeIDs {
		res := results[id]
		nr := nodeResult{Reachable: res.Reachable}
		if res.Reachable {
			nr.Cost = res.Cost
			nr.Path, _ = graphs.Path(results, req.Start, id)
		}
		resp.Results[id] = nr
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json", "error", err)
	}
}
