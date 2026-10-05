package handler

import (
	"slices"
	"fmt"
	"html/template"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/chrishenyard/go-standard-library/graphs"
)

const (
	maxWeight   = 1_000_000
	modeNetwork = "network"
	modeTree    = "tree"
)

var nodeIDs = []string{"A", "B", "C", "D", "E", "F", "G", "H"}

type edgeDef struct {
	From, To string
	Weight   int
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

type edgeView struct {
	Index                  int
	From, To               string
	Weight                 int
	X1, Y1, X2, Y2, MX, MY float64
	Stroke, StrokeWidth    string
	MarkerEnd              string
	BoxStroke, TextFill    string
	OnRoute                bool
}

type nodeView struct {
	ID               string
	X, Y             float64
	Origin, Selected bool
	Fill, Stroke     string
	StrokeWidth      int
	TextFill         string
	LabelFill        string
	Label            string
	Aria             string
	Href             string
}

type distanceView struct {
	ID, Class, Cost string
}

type pageView struct {
	Start, End, Mode string
	Nodes            []string
	NodeViews        []nodeView
	EdgeViews        []edgeView
	AllEdges         []edgeView
	Distances        []distanceView
	Route            string
	Cost             string
	Error            string
	NodeCount        int
	EdgeCount        int
	NetworkURL       string
	TreeURL          string
}

type dijkstraHandler struct {
	tmpl *template.Template
}

func newDijkstraHandler(templatesDir string) (*dijkstraHandler, error) {
	t, err := template.ParseFiles(templatesDir + "/index.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &dijkstraHandler{tmpl: t}, nil
}

func (h *dijkstraHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	view, status := buildPage(r.URL.Query())

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := h.tmpl.Execute(w, view); err != nil {
		slog.ErrorContext(r.Context(), "render dijkstra page", "error", err)
	}
}

func buildPage(q url.Values) (pageView, int) {
	status := http.StatusOK
	if q.Get("reset") != "" {
		q = url.Values{}
	}

	start := pickNode(q.Get("start"), "A")
	end := pickNode(q.Get("end"), "H")
	if d := q.Get("dest"); d != "" {
		end = pickNode(d, end)
	}
	mode := modeNetwork
	if q.Get("mode") == modeTree {
		mode = modeTree
	}

	edges := make([]edgeDef, len(defaultEdges))
	copy(edges, defaultEdges)
	var errMsg string
	for i := range edges {
		raw := q.Get("w" + strconv.Itoa(i))
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > maxWeight {
			errMsg = "Enter a whole-number cost from 0 to 1,000,000."
			status = http.StatusBadRequest
			continue
		}
		edges[i].Weight = n
	}

	g := graphs.New[string]()
	for _, id := range nodeIDs {
		g.AddNode(id)
	}
	for _, e := range edges {
		if err := g.AddEdge(e.From, e.To, e.Weight); err != nil {
			return pageView{Error: err.Error()}, http.StatusInternalServerError
		}
	}
	results, err := g.ShortestPaths(start)
	if err != nil {
		return pageView{Error: err.Error()}, http.StatusInternalServerError
	}

	route, _ := graphs.Path(results, start, end)
	routeEdges := map[string]bool{}
	for i := 1; i < len(route); i++ {
		routeEdges[route[i-1]+"-"+route[i]] = true
	}
	treeEdges := map[string]bool{}
	for _, id := range nodeIDs {
		if r := results[id]; r.Reachable && id != start {
			treeEdges[r.From+"-"+id] = true
		}
	}

	pos := networkPositions
	if mode == modeTree {
		pos = treePositions(results, start)
	}

	// state returns the query string that preserves current edits with overrides.
	state := func(overrides map[string]string) string {
		v := url.Values{"start": {start}, "end": {end}, "mode": {mode}}
		for i, e := range edges {
			v.Set("w"+strconv.Itoa(i), strconv.Itoa(e.Weight))
		}
		for k, val := range overrides {
			v.Set(k, val)
		}
		return "?" + v.Encode()
	}

	view := pageView{
		Start: start, End: end, Mode: mode,
		Nodes:      nodeIDs,
		Route:      "No directed path",
		Cost:       "∞",
		Error:      errMsg,
		NodeCount:  len(nodeIDs),
		EdgeCount:  len(edges),
		NetworkURL: state(map[string]string{"mode": modeNetwork}),
		TreeURL:    state(map[string]string{"mode": modeTree}),
	}
	if len(route) > 0 {
		view.Route = strings.Join(route, " → ")
	}
	if results[end].Reachable {
		view.Cost = strconv.Itoa(results[end].Cost)
	}

	for i, e := range edges {
		key := e.From + "-" + e.To
		onRoute, onTree := routeEdges[key], treeEdges[key]
		ev := edgeView{Index: i, From: e.From, To: e.To, Weight: e.Weight, OnRoute: onRoute}
		if mode == modeTree && !onTree {
			view.AllEdges = append(view.AllEdges, ev)
			continue
		}

		x1, y1 := pos[e.From][0], pos[e.From][1]
		x2, y2 := pos[e.To][0], pos[e.To][1]
		dx, dy := x2-x1, y2-y1
		length := math.Hypot(dx, dy)
		ux, uy := dx/length, dy/length

		offset := 0.0
		if mode == modeNetwork && hasEdge(edges, e.To, e.From) {
			offset = 13
		}
		ev.X1 = x1 + ux*33 - uy*offset
		ev.Y1 = y1 + uy*33 + ux*offset
		ev.X2 = x2 - ux*38 - uy*offset
		ev.Y2 = y2 - uy*38 + ux*offset
		ev.MX, ev.MY = (ev.X1+ev.X2)/2, (ev.Y1+ev.Y2)/2

		switch {
		case onRoute:
			ev.Stroke, ev.StrokeWidth, ev.MarkerEnd = "#ffbb67", "4", "url(#path)"
		case onTree:
			ev.Stroke, ev.StrokeWidth, ev.MarkerEnd = "#5ce5cb", "2.5", "url(#tree)"
		default:
			ev.Stroke, ev.StrokeWidth, ev.MarkerEnd = "#425c68", "1.5", "url(#muted)"
		}
		ev.BoxStroke, ev.TextFill = "#334d59", "#c1d4dc"
		if onRoute {
			ev.BoxStroke, ev.TextFill = "#956f3f", "#ffca87"
		}
		view.AllEdges = append(view.AllEdges, ev)
		view.EdgeViews = append(view.EdgeViews, ev)
	}
	// Route edges are drawn last so they sit on top.
	sort.SliceStable(view.EdgeViews, func(a, b int) bool {
		return !view.EdgeViews[a].OnRoute && view.EdgeViews[b].OnRoute
	})

	onPath := map[string]bool{}
	for _, n := range route {
		onPath[n] = true
	}
	for _, id := range nodeIDs {
		res := results[id]
		nv := nodeView{
			ID: id, X: pos[id][0], Y: pos[id][1],
			Origin: id == start, Selected: id == end,
			Fill: "#19333e", TextFill: "white", LabelFill: "#a8c1cc",
			StrokeWidth: 2, Stroke: "#526674",
			Href: state(map[string]string{"end": id}),
		}
		cost := "unreachable"
		if res.Reachable {
			cost = strconv.Itoa(res.Cost)
			nv.Stroke = "#5b9d96"
		}
		nv.Label = "cost " + cost
		if res.Reachable == false {
			nv.Label = "unreachable"
		}
		if onPath[id] {
			nv.Stroke = "#ffbb67"
			nv.StrokeWidth = 3
		}
		prefix := ""
		if nv.Origin {
			nv.Fill, nv.Stroke, nv.StrokeWidth = "#ffbb67", "#ffcf93", 3
			nv.TextFill, nv.LabelFill = "#172c34", "#ffca87"
			nv.Label = "START · 0"
			prefix = "starting node, "
		} else if nv.Selected {
			nv.Fill = "#264d4e"
		}
		nv.Aria = fmt.Sprintf("Node %s, %scost %s. Select destination.", id, prefix, cost)
		view.NodeViews = append(view.NodeViews, nv)

		d := distanceView{ID: id, Cost: "∞"}
		if res.Reachable {
			d.Cost = strconv.Itoa(res.Cost)
		}
		switch {
		case id == start:
			d.Class = "origin"
		case id == end:
			d.Class = "active"
		}
		view.Distances = append(view.Distances, d)
	}

	return view, status
}

func pickNode(v, fallback string) string {
	if slices.Contains(nodeIDs, v) {
			return v
		}
	return fallback
}

func hasEdge(edges []edgeDef, from, to string) bool {
	for _, e := range edges {
		if e.From == from && e.To == to {
			return true
		}
	}
	return false
}

// treePositions lays nodes out in columns by hop count along the shortest path.
func treePositions(results map[string]graphs.Result[string], start string) map[string][2]float64 {
	layers := map[int][]string{}
	for _, id := range nodeIDs {
		level := len(nodeIDs) - 1
		if p, ok := graphs.Path(results, start, id); ok {
			level = len(p) - 1
		}
		layers[level] = append(layers[level], id)
	}
	levels := make([]int, 0, len(layers))
	for l := range layers {
		levels = append(levels, l)
	}
	sort.Ints(levels)

	out := map[string][2]float64{}
	span := float64(max(1, len(levels)-1))
	for i, l := range levels {
		for j, id := range layers[l] {
			out[id] = [2]float64{
				90 + float64(i)*670/span,
				70 + float64(j+1)*400/float64(len(layers[l])+1),
			}
		}
	}
	return out
}
