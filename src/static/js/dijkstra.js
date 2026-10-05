(() => {
  "use strict";

  const SVG_NS = "http://www.w3.org/2000/svg";
  const graph = JSON.parse(document.getElementById("graph-data").textContent);
  const defaults = { start: "A", end: "H", mode: "network" };

  const $ = (id) => document.getElementById(id);
  const state = {
    ...defaults,
    weights: graph.edges.map((e) => e.weight),
    results: null,
  };
  let requestSeq = 0;

  function svg(tag, attrs, text) {
    const el = document.createElementNS(SVG_NS, tag);
    for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
    if (text !== undefined) el.textContent = text;
    return el;
  }

  function html(tag, props = {}, text) {
    const el = Object.assign(document.createElement(tag), props);
    if (text !== undefined) el.textContent = text;
    return el;
  }

  // The only server callback: Dijkstra runs in Go.
  async function solve() {
    const seq = ++requestSeq;
    try {
      const res = await fetch("/dijkstra", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ start: state.start, weights: state.weights }),
      });
      const body = await res.json();
      if (seq !== requestSeq) return; // a newer request superseded this one
      if (!res.ok) {
        $("error").textContent = body.error || "Request failed.";
        return;
      }
      $("error").textContent = "";
      state.results = body.results;
      render();
    } catch {
      if (seq === requestSeq) $("error").textContent = "Could not reach the server.";
    }
  }

  function treePositions() {
    const layers = new Map();
    for (const id of graph.nodes) {
      const r = state.results[id];
      const level = r.reachable ? r.path.length - 1 : graph.nodes.length - 1;
      if (!layers.has(level)) layers.set(level, []);
      layers.get(level).push(id);
    }
    const levels = [...layers.keys()].sort((a, b) => a - b);
    const span = Math.max(1, levels.length - 1);
    const out = {};
    levels.forEach((l, i) => {
      const ids = layers.get(l);
      ids.forEach((id, j) => {
        out[id] = [90 + (i * 670) / span, 70 + ((j + 1) * 400) / (ids.length + 1)];
      });
    });
    return out;
  }

  function edgeSets() {
    const route = new Set();
    const path = state.results[state.end].path || [];
    for (let i = 1; i < path.length; i++) route.add(`${path[i - 1]}-${path[i]}`);
    const tree = new Set();
    for (const id of graph.nodes) {
      const p = state.results[id].path;
      if (id !== state.start && p && p.length > 1) tree.add(`${p[p.length - 2]}-${id}`);
    }
    return { route, tree, path };
  }

  function render() {
    const { route, tree, path } = edgeSets();
    const pos = state.mode === "tree" ? treePositions() : graph.positions;
    renderGraph(pos, route, tree, path);
    renderRoute(path);
    renderDistances();
    renderEdgeRows(route);
    $("source").textContent = state.start;
    $("start").value = state.start;
    $("end").value = state.end;
    document.querySelectorAll("[data-mode]").forEach((b) => {
      b.setAttribute("aria-pressed", String(b.dataset.mode === state.mode));
    });
  }

  function renderGraph(pos, routeEdges, treeEdges, path) {
    const edgeLayer = $("edge-layer");
    const nodeLayer = $("node-layer");
    edgeLayer.replaceChildren();
    nodeLayer.replaceChildren();

    const drawn = [];
    graph.edges.forEach((e, i) => {
      const key = `${e.from}-${e.to}`;
      const onRoute = routeEdges.has(key);
      const onTree = treeEdges.has(key);
      if (state.mode === "tree" && !onTree) return;

      const [x1, y1] = pos[e.from];
      const [x2, y2] = pos[e.to];
      const dx = x2 - x1;
      const dy = y2 - y1;
      const len = Math.hypot(dx, dy);
      const ux = dx / len;
      const uy = dy / len;
      const hasReverse = graph.edges.some((o) => o.from === e.to && o.to === e.from);
      const offset = state.mode === "network" && hasReverse ? 13 : 0;
      const ax = x1 + ux * 33 - uy * offset;
      const ay = y1 + uy * 33 + ux * offset;
      const bx = x2 - ux * 38 - uy * offset;
      const by = y2 - uy * 38 + ux * offset;

      let stroke = "#425c68";
      let width = "1.5";
      let marker = "url(#muted)";
      if (onRoute) [stroke, width, marker] = ["#ffbb67", "4", "url(#path)"];
      else if (onTree) [stroke, width, marker] = ["#5ce5cb", "2.5", "url(#tree)"];

      drawn.push({ onRoute, i, e, ax, ay, bx, by, stroke, width, marker });
    });
    // Route edges are drawn last so they sit on top.
    drawn.sort((a, b) => Number(a.onRoute) - Number(b.onRoute));

    for (const d of drawn) {
      const mx = (d.ax + d.bx) / 2;
      const my = (d.ay + d.by) / 2;
      edgeLayer.append(
        svg("line", {
          x1: d.ax, y1: d.ay, x2: d.bx, y2: d.by,
          stroke: d.stroke, "stroke-width": d.width, "marker-end": d.marker,
        }),
      );
      const g = svg("g", {
        class: "weight",
        "aria-label": `Cost from ${d.e.from} to ${d.e.to}, currently ${state.weights[d.i]}`,
      });
      g.append(
        svg("rect", {
          x: mx, y: my, width: 36, height: 28, rx: 7, fill: "#172f39",
          stroke: d.onRoute ? "#956f3f" : "#334d59", transform: "translate(-18 -14)",
        }),
        svg("text", {
          x: mx, y: my, dy: 5, "text-anchor": "middle", "font-size": 14,
          fill: d.onRoute ? "#ffca87" : "#c1d4dc",
        }, String(state.weights[d.i])),
      );
      edgeLayer.append(g);
    }

    const onPath = new Set(path);
    for (const id of graph.nodes) {
      const res = state.results[id];
      const [x, y] = pos[id];
      const origin = id === state.start;
      const selected = id === state.end;

      let fill = "#19333e";
      let stroke = res.reachable ? "#5b9d96" : "#526674";
      let strokeWidth = 2;
      let textFill = "white";
      let labelFill = "#a8c1cc";
      let label = res.reachable ? `cost ${res.cost}` : "unreachable";
      let prefix = "";
      if (onPath.has(id)) [stroke, strokeWidth] = ["#ffbb67", 3];
      if (origin) {
        [fill, stroke, strokeWidth, textFill, labelFill] = ["#ffbb67", "#ffcf93", 3, "#172c34", "#ffca87"];
        label = "START · 0";
        prefix = "starting node, ";
      } else if (selected) {
        fill = "#264d4e";
      }

      const a = svg("a", {
        href: "#",
        class: "node",
        "aria-label": `Node ${id}, ${prefix}cost ${res.reachable ? res.cost : "unreachable"}. Select destination.`,
      });
      a.addEventListener("click", (ev) => {
        ev.preventDefault();
        selectEnd(id);
      });
      if (origin) {
        a.append(svg("circle", { cx: x, cy: y, r: 39, fill: "none", stroke: "#ffbb67", opacity: ".25", "stroke-width": 5 }));
      }
      a.append(
        svg("circle", { cx: x, cy: y, r: 30, fill, stroke, "stroke-width": strokeWidth }),
        svg("text", { x, y, dy: 7, "text-anchor": "middle", "font-size": 22, "font-weight": 600, fill: textFill }, id),
        svg("text", { x, y, dy: 53, "text-anchor": "middle", "font-size": 13, fill: labelFill }, label),
      );
      nodeLayer.append(a);
    }
  }

  function renderRoute(path) {
    const r = state.results[state.end];
    $("route").textContent = path.length ? path.join(" → ") : "No directed path";
    $("cost").textContent = r.reachable ? String(r.cost) : "∞";
  }

  function renderDistances() {
    const frag = document.createDocumentFragment();
    for (const id of graph.nodes) {
      const r = state.results[id];
      let cls = "distance";
      if (id === state.start) cls += " origin";
      else if (id === state.end) cls += " active";
      const b = html("button", { type: "button", className: cls });
      b.setAttribute("aria-label", `Inspect path to ${id}`);
      b.append(html("span", {}, id), html("strong", {}, r.reachable ? String(r.cost) : "∞"));
      b.addEventListener("click", () => selectEnd(id));
      frag.append(b);
    }
    $("distances").replaceChildren(frag);
  }

  // Rows are rebuilt only to refresh highlighting; skip the focused input to keep typing intact.
  function renderEdgeRows(route) {
    const rows = $("edges").children;
    graph.edges.forEach((e, i) => {
      const row = rows[i];
      row.classList.toggle("selected", route.has(`${e.from}-${e.to}`));
      const input = row.querySelector("input");
      if (document.activeElement !== input) input.value = state.weights[i];
    });
  }

  function buildStaticControls() {
    for (const sel of [$("start"), $("end")]) {
      sel.replaceChildren(...graph.nodes.map((id) => html("option", { value: id }, id)));
    }
    $("graphmeta").textContent = `${graph.nodes.length} nodes / ${graph.edges.length} edges`;

    const frag = document.createDocumentFragment();
    graph.edges.forEach((e, i) => {
      const label = html("label", { className: "edgerow" });
      const input = html("input", {
        type: "number", min: 0, max: graph.maxWeight, step: 1, name: `w${i}`,
      });
      input.setAttribute("aria-label", `Cost from ${e.from} to ${e.to}`);
      input.addEventListener("change", () => onWeightChange(i, input));
      const name = html("span", {}, e.from);
      name.append(html("b", {}, "→"), e.to);
      label.append(name, input);
      frag.append(label);
    });
    $("edges").replaceChildren(frag);
  }

  function onWeightChange(i, input) {
    const n = Number(input.value);
    if (input.value === "" || !Number.isInteger(n) || n < 0 || n > graph.maxWeight) {
      $("error").textContent = "Enter a whole-number cost from 0 to 1,000,000.";
      return;
    }
    state.weights[i] = n;
    solve();
  }

  function selectEnd(id) {
    state.end = id;
    render();
  }

  function reset() {
    Object.assign(state, defaults, { weights: graph.edges.map((e) => e.weight) });
    $("error").textContent = "";
    solve();
  }

  $("start").addEventListener("change", (ev) => {
    state.start = ev.target.value;
    solve();
  });
  $("end").addEventListener("change", (ev) => selectEnd(ev.target.value));
  $("reset").addEventListener("click", reset);
  document.querySelectorAll("[data-mode]").forEach((b) => {
    b.addEventListener("click", () => {
      state.mode = b.dataset.mode;
      render();
    });
  });

  buildStaticControls();
  solve();
})();
