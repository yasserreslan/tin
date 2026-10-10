// Code views: a highlighted file with line numbers and line links, inline code blocks for Markdown, and the diff
// views (unified and split, with word-level marks).

import { h, clear } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as hl from "../lib/highlight.js";
import * as df from "../lib/diff.js";
import * as md from "../lib/markdown.js";
import { sanitize } from "../lib/sanitize.js";

// tokens renders one line's tokens.
export function tokens(toks) {
	if (!toks.length) return ["​"];
	return toks.map(([cls, v]) => (cls ? h("span", { class: cls }, v) : v));
}

// highlightBlock is a <code> with highlighted lines, for Markdown code blocks.
export function highlightBlock(lang, text) {
	const map = { tin: "tin", go: "go", js: "js", javascript: "js", ts: "js", typescript: "js", sh: "shell", bash: "shell", shell: "shell", console: "shell", json: "json", sql: "sql", py: "python", python: "python", css: "css", html: "html", yaml: "yaml", yml: "yaml", toml: "yaml", c: "c", rust: "c", rs: "c" };
	const l = map[(lang || "").toLowerCase()];
	if (!l) return null;
	const rows = hl.lines(text, l);
	return h("code", {}, rows.map((r, i) => [tokens(r), i < rows.length - 1 ? "\n" : ""]));
}

// markdown renders Markdown source into a .md element. base resolves relative links and images
// ({link(path), image(path)}).
export function markdown(src, base = {}) {
	const html = (raw) => sanitize(raw, { link: base.link, image: base.image });
	return h("div.md", {}, md.render(md.parse(src), h, { code: highlightBlock, link: base.link, image: base.image, html }));
}

// fileView is a file's lines with numbers. Clicking a number selects the line (shift: a range) and calls onSelect.
export function fileView(text, { lang = "", selected = null, onSelect, wrap = false, maxLines = 20000 } = {}) {
	const rows = hl.lines(text, lang);
	const shown = rows.length > maxLines ? rows.slice(0, maxLines) : rows;
	const tbody = h("tbody");
	let anchor = selected ? selected[0] : 0;
	const trs = shown.map((toks, i) => {
		const n = i + 1;
		const tr = h("tr", { id: "L" + n }, h("td.ln", { dataset: { n: String(n) } }, String(n)), h("td.lc", {}, tokens(toks)));
		return tr;
	});
	tbody.append(...trs);
	const mark = (sel) => {
		for (const tr of tbody.querySelectorAll("tr.hl")) tr.classList.remove("hl");
		if (!sel) return;
		for (let i = sel[0]; i <= sel[1]; i++) if (trs[i - 1]) trs[i - 1].classList.add("hl");
	};
	mark(selected);
	tbody.addEventListener("click", (e) => {
		const td = e.target.closest("td.ln");
		if (!td) return;
		const n = Number(td.dataset.n);
		let sel = [n, n];
		if (e.shiftKey && anchor) sel = [Math.min(anchor, n), Math.max(anchor, n)];
		else anchor = n;
		mark(sel);
		if (onSelect) onSelect(sel);
	});
	const wrapEl = h("div", { class: ["code-wrap", wrap && "wrap"] }, h("table.code", {}, tbody));
	if (rows.length > maxLines) wrapEl.appendChild(h("div.box-empty", {}, `Showing the first ${maxLines} of ${rows.length} lines.`));
	wrapEl.scrollToLine = (n) => {
		const tr = trs[n - 1];
		if (tr) tr.scrollIntoView({ block: "center" });
	};
	return wrapEl;
}

// parseLines reads "L10" or "L10-L20" into [10, 20].
export function parseLines(hash) {
	const m = /^L(\d+)(?:-L?(\d+))?$/.exec(hash || "");
	if (!m) return null;
	const a = Number(m[1]);
	const b = m[2] ? Number(m[2]) : a;
	return [Math.min(a, b), Math.max(a, b)];
}

// ---- diffs ----

function segs(list) {
	return list.map(([changed, t]) => (changed ? h("span", { class: "x" }, t) : t));
}

// highlightRow renders a diff row's text, highlighted by lang, or with word marks when given.
function rowText(text, lang, marks, op) {
	if (marks) return marks.map(([changed, t]) => (changed ? h(op === "-" ? "del" : "ins", {}, t) : t));
	if (!lang) return text || "​";
	const toks = hl.lines(text, lang)[0] || [];
	return tokens(toks);
}

// wordMarks pairs each run of removed lines with the added lines after it and marks the changed words.
function wordMarks(rows) {
	const marks = new Map();
	let i = 0;
	while (i < rows.length) {
		if (rows[i].op !== "-") {
			i++;
			continue;
		}
		const dels = [];
		const adds = [];
		while (i < rows.length && rows[i].op === "-") dels.push(rows[i++]);
		while (i < rows.length && rows[i].op === "+") adds.push(rows[i++]);
		if (dels.length === adds.length && dels.length <= 8) {
			for (let k = 0; k < dels.length; k++) {
				const w = df.words(dels[k].text, adds[k].text);
				if (w.a.length > 1 || w.b.length > 1) {
					marks.set(dels[k], w.a);
					marks.set(adds[k], w.b);
				}
			}
		}
	}
	return marks;
}

// hunkTable renders hunks in a unified or split table. opts.onComment(row, side) adds a comment button on each line;
// opts.rowExtra(row) returns comment rows to show under a line.
export function hunkTable(hunkList, { lang = "", split = false, onComment, rowExtra } = {}) {
	const table = h("table", { class: ["diff", split && "split"] });
	const colgroup = split
		? h("colgroup", {}, h("col", { style: { width: "52px" } }), h("col", { style: { width: "18px" } }), h("col"), h("col", { style: { width: "52px" } }), h("col", { style: { width: "18px" } }), h("col"))
		: h("colgroup", {}, h("col", { style: { width: "52px" } }), h("col", { style: { width: "52px" } }), h("col", { style: { width: "18px" } }), h("col"));
	table.appendChild(colgroup);
	const tbody = h("tbody");
	const cols = split ? 6 : 4;
	for (const hk of hunkList) {
		tbody.appendChild(h("tr.hunk", {}, h("td", { colspan: cols }, `@@ -${hk.aStart},${hk.aLen} +${hk.bStart},${hk.bLen} @@`, hk.header ? h("span.faint", {}, "  " + hk.header) : null)));
		const marks = wordMarks(hk.rows);
		const commentBtn = (row) => (onComment ? h("button.add-comment", { type: "button", title: "Comment on this line", onclick: () => onComment(row) }, icon("plus", "sm")) : null);
		if (!split) {
			for (const r of hk.rows) {
				const cls = r.op === "+" ? "add" : r.op === "-" ? "del" : "";
				tbody.appendChild(h("tr", { class: cls }, h("td.ln", {}, r.a ? String(r.a) : ""), h("td.ln", {}, r.b ? String(r.b) : ""), h("td.sign", {}, commentBtn(r), r.op === " " ? "" : r.op), h("td.lc", {}, rowText(r.text, lang, marks.get(r), r.op))));
				if (rowExtra) {
					const extra = rowExtra(r);
					if (extra) tbody.appendChild(h("tr.comment-row", {}, h("td", { colspan: cols }, extra)));
				}
			}
		} else {
			for (const p of df.sideBySide(hk.rows)) {
				const L = p.left;
				const Rr = p.right;
				const same = L && Rr && L === Rr;
				const lcls = !L ? "e" : same ? "" : "d";
				const rcls = !Rr ? "e" : same ? "" : "a";
				tbody.appendChild(
					h(
						"tr",
						{},
						h("td", { class: ["ln", lcls] }, L ? String(L.a) : ""),
						h("td", { class: ["sign", lcls] }, L && !same ? "-" : ""),
						h("td", { class: ["lc", lcls] }, L ? rowText(L.text, lang, marks.get(L), "-") : ""),
						h("td", { class: ["ln", rcls] }, Rr ? String(Rr.b) : ""),
						h("td", { class: ["sign", rcls] }, commentBtn(Rr || L), Rr && !same ? "+" : ""),
						h("td", { class: ["lc", rcls] }, Rr ? rowText(Rr.text, lang, marks.get(Rr), "+") : ""),
					),
				);
				if (rowExtra) {
					const extra = rowExtra(Rr || L);
					if (extra) tbody.appendChild(h("tr.comment-row", {}, h("td", { colspan: cols }, extra)));
				}
			}
		}
	}
	table.appendChild(tbody);
	return table;
}

// diffbar is five squares showing the share of added and removed lines.
export function diffbar(add, del) {
	const total = add + del;
	const n = 5;
	let a = total ? Math.round((add / total) * n) : 0;
	let d = total ? Math.round((del / total) * n) : 0;
	if (a + d > n) d = n - a;
	return h("span.diffbar", {}, Array.from({ length: n }, (_, i) => h("i", { class: i < a ? "a" : i < a + d ? "d" : "" })));
}

// fileBlock is one file's diff box with a sticky head; body is built lazily when first expanded.
export function fileBlock({ path, oldPath, kind, add, del, build, collapsed = false, actions = [], badge }) {
	const body = h("div.diff-body");
	let built = false;
	const ensure = () => {
		if (built) return;
		built = true;
		const content = build();
		if (content instanceof Promise) {
			body.appendChild(h("div.loading", {}, h("span.spinner"), "Loading…"));
			content.then(
				(c) => {
					clear(body);
					body.appendChild(c);
				},
				(err) => {
					clear(body);
					body.appendChild(h("div.box-empty", {}, err.message || String(err)));
				},
			);
		} else body.appendChild(content);
	};
	const toggle = h("button.btn.ghost.icon.sm", { type: "button", "aria-label": "Collapse" }, icon(collapsed ? "chevronRight" : "chevronDown", "sm"));
	const el = h(
		"div",
		{ class: ["diff-file", collapsed && "collapsed"], id: "diff-" + encodeURIComponent(path) },
		h(
			"div.diff-file-head",
			{},
			toggle,
			kind && kind !== "modified" ? h("span", { class: ["badge", kind === "added" ? "green" : kind === "deleted" ? "red" : "blue"] }, kind) : null,
			h("span.path.ellipsis", {}, oldPath && oldPath !== path ? [h("span.muted", {}, oldPath), " → "] : null, path),
			badge || null,
			h("span.spacer"),
			add !== undefined ? [h("span.stat-add", {}, "+" + add), h("span.stat-del", {}, "−" + del), diffbar(add, del)] : null,
			actions,
		),
		body,
	);
	toggle.onclick = () => {
		const c = el.classList.toggle("collapsed");
		toggle.replaceChildren(icon(c ? "chevronRight" : "chevronDown", "sm"));
		if (!c) ensure();
	};
	if (!collapsed) ensure();
	el.expand = () => {
		if (el.classList.contains("collapsed")) toggle.click();
	};
	return el;
}
