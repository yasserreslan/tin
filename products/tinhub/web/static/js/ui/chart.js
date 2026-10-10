// Charts in SVG: a line chart with points, hover tips and an optional threshold band, and sparklines.

import { h, s } from "../lib/dom.js";
import * as fmt from "../lib/format.js";

const PALETTE = ["var(--accent)", "var(--green)", "var(--purple)", "var(--amber)", "var(--red)", "var(--blue)"];

export function color(i) {
	return PALETTE[i % PALETTE.length];
}

let tip = null;

function showTip(e, lines) {
	if (!tip) {
		tip = h("div.chart-tip");
		document.body.appendChild(tip);
	}
	tip.replaceChildren(...lines.map((l, i) => h("div", { style: i === 0 ? { "font-weight": "600" } : null }, l)));
	tip.style.display = "block";
	const x = Math.min(e.clientX + 14, window.innerWidth - tip.offsetWidth - 8);
	const y = Math.min(e.clientY + 14, window.innerHeight - tip.offsetHeight - 8);
	tip.style.left = x + "px";
	tip.style.top = y + "px";
}

function hideTip() {
	if (tip) tip.style.display = "none";
}

function niceTicks(min, max, n = 4) {
	if (min === max) {
		const d = Math.abs(min) * 0.1 || 1;
		min -= d;
		max += d;
	}
	const span = max - min;
	const step0 = span / n;
	const mag = 10 ** Math.floor(Math.log10(step0));
	const step = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((x) => x >= step0) || step0;
	const lo = Math.floor(min / step) * step;
	const hi = Math.ceil(max / step) * step;
	const ticks = [];
	for (let v = lo; v <= hi + step / 2; v += step) ticks.push(Number(v.toPrecision(12)));
	return { lo, hi, ticks };
}

// line draws series [{label, color, points: [{x, y, tip: [lines], onclick}]}]; x is an index or a time.
export function line(series, { height = 260, unit = "", yLabel = (v) => fmt.metric(v, unit), xLabel = (x) => fmt.date(x), zero = false } = {}) {
	const W = 900;
	const H = height;
	const pad = { l: 64, r: 16, t: 14, b: 28 };
	const all = series.flatMap((sr) => sr.points);
	if (!all.length) return h("div.box-empty", {}, "No points yet.");
	const xs = all.map((p) => p.x);
	const ys = all.map((p) => p.y);
	let x0 = Math.min(...xs);
	let x1 = Math.max(...xs);
	if (x0 === x1) {
		x0 -= 1;
		x1 += 1;
	}
	const yt = niceTicks(zero ? Math.min(0, ...ys) : Math.min(...ys), Math.max(...ys));
	const X = (x) => pad.l + ((x - x0) / (x1 - x0)) * (W - pad.l - pad.r);
	const Y = (y) => pad.t + (1 - (y - yt.lo) / (yt.hi - yt.lo || 1)) * (H - pad.t - pad.b);
	const svg = s("svg", { class: "chart", viewBox: `0 0 ${W} ${H}`, role: "img" });
	const grid = s("g", { class: "grid" });
	const axis = s("g", { class: "axis" });
	for (const t of yt.ticks) {
		grid.appendChild(s("line", { x1: pad.l, x2: W - pad.r, y1: Y(t), y2: Y(t) }));
		axis.appendChild(s("text", { x: pad.l - 8, y: Y(t) + 4, "text-anchor": "end" }, yLabel(t)));
	}
	const nx = Math.min(6, new Set(xs).size);
	for (let i = 0; i < nx; i++) {
		const xv = x0 + ((x1 - x0) * i) / Math.max(1, nx - 1);
		axis.appendChild(s("text", { x: X(xv), y: H - 8, "text-anchor": i === 0 ? "start" : i === nx - 1 ? "end" : "middle" }, xLabel(xv)));
	}
	svg.append(grid, axis);
	series.forEach((sr, si) => {
		const c = sr.color || color(si);
		const pts = [...sr.points].sort((a, b) => a.x - b.x);
		if (!pts.length) return;
		const d = pts.map((p, i) => `${i ? "L" : "M"}${X(p.x).toFixed(1)},${Y(p.y).toFixed(1)}`).join(" ");
		const area = d + ` L${X(pts[pts.length - 1].x).toFixed(1)},${H - pad.b} L${X(pts[0].x).toFixed(1)},${H - pad.b} Z`;
		svg.appendChild(s("path", { d: area, class: "area", fill: c }));
		svg.appendChild(s("path", { d, class: "series", stroke: c }));
		for (const p of pts) {
			svg.appendChild(
				s("circle", {
					class: p.onclick ? "pt link" : "pt",
					cx: X(p.x),
					cy: Y(p.y),
					r: pts.length > 80 ? 2.5 : 4,
					fill: p.color || c,
					onmousemove: (e) => showTip(e, p.tip || [sr.label, yLabel(p.y)]),
					onmouseleave: hideTip,
					onclick: p.onclick || null,
				}),
			);
		}
	});
	return svg;
}

// spark is a tiny line of values.
export function spark(values, { width = 120, height = 28 } = {}) {
	if (!values.length) return s("svg", { class: "spark", viewBox: `0 0 ${width} ${height}` });
	const lo = Math.min(...values);
	const hi = Math.max(...values);
	const X = (i) => (values.length === 1 ? width / 2 : (i / (values.length - 1)) * (width - 4) + 2);
	const Y = (v) => height - 3 - ((v - lo) / (hi - lo || 1)) * (height - 6);
	const d = values.map((v, i) => `${i ? "L" : "M"}${X(i).toFixed(1)},${Y(v).toFixed(1)}`).join(" ");
	return s("svg", { class: "spark", viewBox: `0 0 ${width} ${height}` }, s("path", { d }));
}

// bars is a horizontal bar list: [{label, value, color, sub}].
export function bars(items, { format = (v) => fmt.count(v) } = {}) {
	const max = Math.max(1, ...items.map((i) => i.value));
	return h(
		"div.col",
		{ style: { gap: "10px" } },
		items.map((it, i) =>
			h(
				"div",
				{},
				h("div.row", { style: { "font-size": "12.5px", "margin-bottom": "4px" } }, h("span.grow.ellipsis", {}, it.label), h("span.muted.mono", {}, format(it.value))),
				h("div.progress", {}, h("div", { style: { width: `${(it.value / max) * 100}%`, background: it.color || color(i) } })),
				it.sub ? h("div.tiny.faint", { style: { "margin-top": "2px" } }, it.sub) : null,
			),
		),
	);
}
