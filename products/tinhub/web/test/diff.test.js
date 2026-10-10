// Line diffs (web/static/js/lib/diff.js).
import { test } from "node:test";
import assert from "node:assert/strict";
import { splitLines, diffLines, hunks, stats, parseUnified, words, sideBySide } from "../static/js/lib/diff.js";

const apply = (a, rows) => rows.filter((r) => r.op !== "-").map((r) => r.text);

test("splitLines", () => {
	assert.deepEqual(splitLines("a\nb\n"), ["a", "b"]);
	assert.deepEqual(splitLines("a\r\nb"), ["a", "b"]);
	assert.deepEqual(splitLines(""), []);
});

test("diffLines makes b from a", () => {
	const cases = [
		["a b c d e", "a b x d e"],
		["", "a b"],
		["a b", ""],
		["a b c", "c b a"],
		["x a b c y", "a b c"],
		["1 2 3 4 5 6 7 8 9", "1 2 4 5 6 7 9 10"],
	];
	for (const [x, y] of cases) {
		const a = x ? x.split(" ") : [];
		const b = y ? y.split(" ") : [];
		const rows = diffLines(a, b);
		assert.deepEqual(apply(a, rows), b, `${x} -> ${y}`);
		assert.deepEqual(
			rows.filter((r) => r.op !== "+").map((r) => r.text),
			a,
		);
		const s = stats(rows);
		assert.equal(rows.filter((r) => r.op === " ").length + s.del, a.length);
	}
});

test("diffLines numbers lines", () => {
	const rows = diffLines(["a", "b", "c"], ["a", "x", "c"]);
	assert.deepEqual(
		rows.map((r) => `${r.op}${r.a}:${r.b}`),
		[" 1:1", "-2:0", "+0:2", " 3:3"],
	);
});

test("random edits round-trip", () => {
	let seed = 7;
	const rnd = (n) => ((seed = (seed * 1103515245 + 12345) % 2147483648), seed % n);
	for (let i = 0; i < 200; i++) {
		const a = Array.from({ length: rnd(30) }, () => "l" + rnd(8));
		const b = a.filter(() => rnd(4)).flatMap((x) => (rnd(5) ? [x] : [x, "n" + rnd(8)]));
		assert.deepEqual(apply(a, diffLines(a, b)), b);
	}
});

test("hunks keep context and merge near changes", () => {
	const a = Array.from({ length: 30 }, (_, i) => "l" + i);
	const b = [...a];
	b[5] = "x";
	b[8] = "y";
	b[25] = "z";
	const h = hunks(diffLines(a, b), 3);
	assert.equal(h.length, 2);
	assert.equal(h[0].aStart, 3);
	assert.equal(h[1].bStart, 23);
});

test("parseUnified", () => {
	const h = parseUnified("--- a\n+++ b\n@@ -1,3 +1,4 @@ fn main\n a\n-b\n+c\n+d\n e\n\\ No newline at end of file\n");
	assert.equal(h.length, 1);
	assert.equal(h[0].header, "fn main");
	assert.deepEqual(
		h[0].rows.map((r) => `${r.op}${r.a}:${r.b}:${r.text}`),
		[" 1:1:a", "-2:0:b", "+0:2:c", "+0:3:d", " 3:4:e"],
	);
	assert.equal(parseUnified("@@ -0,0 +1,2 @@\n+x\n+y")[0].rows[1].b, 2);
});

test("words marks the changed part", () => {
	const w = words("let total = price * count", "let total = price * amount");
	assert.deepEqual(
		w.b.filter(([c]) => c).map(([, t]) => t),
		["amount"],
	);
	const all = words("abc", "xyz");
	assert.deepEqual(all.a, [[true, "abc"]]);
});

test("sideBySide pairs removals with additions", () => {
	const rows = diffLines(["a", "b", "c"], ["a", "x", "y", "c"]);
	const s = sideBySide(rows);
	assert.equal(s.length, 4);
	assert.equal(s[1].left.text, "b");
	assert.equal(s[1].right.text, "x");
	assert.equal(s[2].left, null);
});
