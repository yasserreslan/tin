// Line diffs with no DOM: diffLines (Myers, O((N+M)D)) makes edit rows, hunks groups them with context, parseUnified
// reads a unified diff's text (the semantic diffs' per-declaration bodies), and words marks the changed parts of a
// changed line pair. Tested by web/test/diff.test.js.

// splitLines splits text into lines without their newlines; a trailing newline adds no empty line.
export function splitLines(text) {
	if (text === "" || text === null || text === undefined) return [];
	const rows = String(text).replace(/\r\n?/g, "\n").split("\n");
	if (rows[rows.length - 1] === "") rows.pop();
	return rows;
}

// diffLines is the edit script from a to b: rows {op: " " | "-" | "+", a: line number in a or 0, b: in b or 0, text}.
export function diffLines(a, b, limit = 4000) {
	const n = a.length;
	const m = b.length;
	// trim the common head and tail first: the usual change is small
	let head = 0;
	while (head < n && head < m && a[head] === b[head]) head++;
	let tail = 0;
	while (tail < n - head && tail < m - head && a[n - 1 - tail] === b[m - 1 - tail]) tail++;
	const A = a.slice(head, n - tail);
	const B = b.slice(head, m - tail);
	const mid = A.length * B.length > limit * limit ? coarse(A, B) : myers(A, B);
	const rows = [];
	for (let i = 0; i < head; i++) rows.push({ op: " ", a: i + 1, b: i + 1, text: a[i] });
	for (const e of mid) rows.push({ op: e.op, a: e.a ? e.a + head : 0, b: e.b ? e.b + head : 0, text: e.text });
	for (let i = 0; i < tail; i++) rows.push({ op: " ", a: n - tail + i + 1, b: m - tail + i + 1, text: a[n - tail + i] });
	return rows;
}

// coarse is the diff of inputs too large for Myers: all of a removed, all of b added.
function coarse(A, B) {
	return [...A.map((t, i) => ({ op: "-", a: i + 1, b: 0, text: t })), ...B.map((t, i) => ({ op: "+", a: 0, b: i + 1, text: t }))];
}

function myers(A, B) {
	const n = A.length;
	const m = B.length;
	if (n === 0) return B.map((t, i) => ({ op: "+", a: 0, b: i + 1, text: t }));
	if (m === 0) return A.map((t, i) => ({ op: "-", a: i + 1, b: 0, text: t }));
	const max = n + m;
	const off = max;
	const v = new Int32Array(2 * max + 2);
	const trace = [];
	let found = false;
	for (let d = 0; d <= max && !found; d++) {
		trace.push(v.slice());
		for (let k = -d; k <= d; k += 2) {
			let x;
			if (k === -d || (k !== d && v[off + k - 1] < v[off + k + 1])) x = v[off + k + 1];
			else x = v[off + k - 1] + 1;
			let y = x - k;
			while (x < n && y < m && A[x] === B[y]) {
				x++;
				y++;
			}
			v[off + k] = x;
			if (x >= n && y >= m) {
				found = true;
				break;
			}
		}
	}
	// walk back through the trace
	const out = [];
	let x = n;
	let y = m;
	for (let d = trace.length - 1; d >= 0; d--) {
		const vv = trace[d];
		const k = x - y;
		let prevK;
		if (k === -d || (k !== d && vv[off + k - 1] < vv[off + k + 1])) prevK = k + 1;
		else prevK = k - 1;
		const prevX = vv[off + prevK];
		const prevY = prevX - prevK;
		while (x > prevX && y > prevY) {
			out.push({ op: " ", a: x, b: y, text: A[x - 1] });
			x--;
			y--;
		}
		if (d > 0) {
			if (x === prevX) out.push({ op: "+", a: 0, b: y, text: B[y - 1] });
			else out.push({ op: "-", a: x, b: 0, text: A[x - 1] });
		}
		x = prevX;
		y = prevY;
	}
	return out.reverse();
}

// hunks groups rows into hunks with ctx lines of context: {aStart, aLen, bStart, bLen, rows}.
export function hunks(rows, ctx = 3) {
	const out = [];
	let cur = null;
	let lastChange = -Infinity;
	for (let i = 0; i < rows.length; i++) {
		if (rows[i].op === " ") continue;
		const from = Math.max(0, i - ctx);
		if (cur && from <= lastChange + ctx + 1) {
			for (let j = lastChange + 1; j <= i; j++) cur.rows.push(rows[j]);
		} else {
			if (cur) close(cur, rows, lastChange, ctx, out);
			cur = { rows: rows.slice(from, i + 1) };
		}
		lastChange = i;
	}
	if (cur) close(cur, rows, lastChange, ctx, out);
	return out;
}

function close(cur, rows, lastChange, ctx, out) {
	for (let j = lastChange + 1; j < Math.min(rows.length, lastChange + 1 + ctx); j++) cur.rows.push(rows[j]);
	const firstA = cur.rows.find((r) => r.a);
	const firstB = cur.rows.find((r) => r.b);
	cur.aStart = firstA ? firstA.a : 0;
	cur.bStart = firstB ? firstB.b : 0;
	cur.aLen = cur.rows.filter((r) => r.op !== "+").length;
	cur.bLen = cur.rows.filter((r) => r.op !== "-").length;
	out.push(cur);
}

// stats counts added and removed rows.
export function stats(rows) {
	let add = 0;
	let del = 0;
	for (const r of rows) {
		if (r.op === "+") add++;
		else if (r.op === "-") del++;
	}
	return { add, del };
}

// parseUnified reads unified diff text into hunks like hunks() makes; lines before the first @@ are skipped.
export function parseUnified(text) {
	const out = [];
	let cur = null;
	let a = 0;
	let b = 0;
	for (const line of splitLines(text)) {
		const m = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)$/.exec(line);
		if (m) {
			a = Number(m[1]);
			b = Number(m[3]);
			cur = { aStart: a, aLen: Number(m[2] ?? 1), bStart: b, bLen: Number(m[4] ?? 1), header: m[5].trim(), rows: [] };
			out.push(cur);
			continue;
		}
		if (!cur) {
			if (line.startsWith("---") || line.startsWith("+++") || line.startsWith("diff ") || line.startsWith("index ")) continue;
			// a body with no hunk header: every line is context, numbered from 1
			cur = { aStart: 1, aLen: 0, bStart: 1, bLen: 0, header: "", rows: [] };
			a = 1;
			b = 1;
			out.push(cur);
		}
		if (line.startsWith("\\")) continue;
		const op = line[0] === "+" || line[0] === "-" ? line[0] : " ";
		const t = line[0] === "+" || line[0] === "-" || line[0] === " " ? line.slice(1) : line;
		if (op === "+") cur.rows.push({ op, a: 0, b: b++, text: t });
		else if (op === "-") cur.rows.push({ op, a: a++, b: 0, text: t });
		else cur.rows.push({ op, a: a++, b: b++, text: t });
	}
	return out;
}

// tokens splits a line into words, spaces and punctuation, for words().
function tokens(s) {
	return s.match(/[A-Za-z0-9_]+|\s+|./gu) || [];
}

// words compares a removed and an added line: {a, b}, each a list of [changed, text] segments. When the lines
// share less than a third, everything is changed (marking would only be noise).
export function words(oldLine, newLine) {
	const ta = tokens(oldLine);
	const tb = tokens(newLine);
	if (ta.length * tb.length > 250000) return { a: [[true, oldLine]], b: [[true, newLine]] };
	const rows = myers(ta, tb);
	const a = [];
	const b = [];
	let same = 0;
	const pushSeg = (list, changed, t) => {
		const last = list[list.length - 1];
		if (last && last[0] === changed) last[1] += t;
		else list.push([changed, t]);
	};
	for (const r of rows) {
		if (r.op === " ") {
			pushSeg(a, false, r.text);
			pushSeg(b, false, r.text);
			same += r.text.length;
		} else if (r.op === "-") pushSeg(a, true, r.text);
		else pushSeg(b, true, r.text);
	}
	if (same * 3 < Math.max(oldLine.length, newLine.length)) return { a: [[true, oldLine]], b: [[true, newLine]] };
	return { a, b };
}

// pairs splits a hunk's rows into display rows for a side-by-side view: [{left, right}], each a row or null.
export function sideBySide(rows) {
	const out = [];
	let i = 0;
	while (i < rows.length) {
		if (rows[i].op === " ") {
			out.push({ left: rows[i], right: rows[i] });
			i++;
			continue;
		}
		const dels = [];
		const adds = [];
		while (i < rows.length && rows[i].op === "-") dels.push(rows[i++]);
		while (i < rows.length && rows[i].op === "+") adds.push(rows[i++]);
		const k = Math.max(dels.length, adds.length);
		for (let j = 0; j < k; j++) out.push({ left: dels[j] || null, right: adds[j] || null });
	}
	return out;
}
