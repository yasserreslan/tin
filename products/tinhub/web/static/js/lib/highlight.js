// Syntax highlighting with no DOM: lines(text, lang) splits source into lines of [class, text] tokens, carrying
// multi-line strings and comments across lines. Tin is first-class (keywords of edition 1, unit literals, string
// interpolation, attributes); a few other languages get keywords, strings, numbers and comments.

const words = (s) => new Set(s.split(/\s+/).filter(Boolean));

const TIN = {
	keywords: words("break catch const continue defer detach else enum fail fn for guard if import keep let limit match mut package parallel return secret shared struct try type within in on use with once select scope arena shape dyn wrap test bench"),
	constants: words("true false nil"),
	types: words("i8 i16 i32 i64 u8 u16 u32 u64 f32 f64 bool str rune byte fault map any"),
	builtins: words("len cap append make new copy delete min max panic fault keep bound reveal print println after canceled"),
	line: "//",
	block: null,
	quotes: ['"', "'"],
	raw: "`",
	interp: true,
	attrs: true,
	units: true,
};

const GO = {
	keywords: words("break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var"),
	constants: words("true false nil iota"),
	types: words("bool byte complex64 complex128 error float32 float64 int int8 int16 int32 int64 rune string uint uint8 uint16 uint32 uint64 uintptr any"),
	builtins: words("append cap close complex copy delete imag len make new panic print println real recover min max clear"),
	line: "//",
	block: ["/*", "*/"],
	quotes: ['"', "'"],
	raw: "`",
};

const JS = {
	keywords: words("async await break case catch class const continue debugger default delete do else export extends finally for from function if import in instanceof let new of return static super switch this throw try typeof var void while with yield interface type implements enum as"),
	constants: words("true false null undefined NaN Infinity"),
	types: words("string number boolean any unknown never object"),
	builtins: words("console document window Math JSON Object Array Promise Map Set Date Number String Error"),
	line: "//",
	block: ["/*", "*/"],
	quotes: ['"', "'"],
	raw: "`",
};

const C = {
	keywords: words("auto break case const continue default do else enum extern for goto if inline register restrict return sizeof static struct switch typedef union volatile while class public private protected namespace template typename virtual fn let mut impl trait pub use mod match loop where self Self crate"),
	constants: words("true false NULL nullptr null None"),
	types: words("char double float int long short signed unsigned void bool size_t uint8_t uint16_t uint32_t uint64_t int8_t int16_t int32_t int64_t i8 i16 i32 i64 u8 u16 u32 u64 f32 f64 usize isize str String"),
	builtins: words(""),
	line: "//",
	block: ["/*", "*/"],
	quotes: ['"', "'"],
	raw: null,
	preproc: true,
};

const SHELL = {
	keywords: words("if then else elif fi for while until do done case esac in function return exit export local set unset shift break continue readonly trap eval exec source"),
	constants: words("true false"),
	types: words(""),
	builtins: words("echo printf cd ls cat grep sed awk test read mkdir rm cp mv curl make tit tinhub tin"),
	line: "#",
	block: null,
	quotes: ['"', "'"],
	raw: null,
	vars: true,
};

const PY = {
	keywords: words("and as assert async await break class continue def del elif else except finally for from global if import in is lambda nonlocal not or pass raise return try while with yield"),
	constants: words("True False None"),
	types: words("int float str bytes bool list dict set tuple object"),
	builtins: words("print len range open super isinstance enumerate zip map filter sorted"),
	line: "#",
	block: null,
	quotes: ['"', "'"],
	raw: null,
};

const SQL = {
	keywords: words("select from where and or not insert into values update set delete create table index on primary key references join left right inner outer group by order having limit offset as distinct union all returning with case when then else end conflict do nothing is null in exists alter add drop column default unique begin commit rollback coalesce"),
	constants: words("true false null"),
	types: words("bigint int integer text varchar boolean timestamptz timestamp bytea jsonb serial bigserial real double"),
	builtins: words("now count sum max min avg extract"),
	line: "--",
	block: ["/*", "*/"],
	quotes: ["'"],
	raw: null,
	ci: true,
};

const CSS = {
	keywords: words("@media @import @keyframes @font-face @supports important"),
	constants: words(""),
	types: words(""),
	builtins: words("var calc rgba rgb hsl url linear-gradient radial-gradient color-mix"),
	line: null,
	block: ["/*", "*/"],
	quotes: ['"', "'"],
	raw: null,
};

const YAML = {
	keywords: words(""),
	constants: words("true false null yes no on off"),
	types: words(""),
	builtins: words(""),
	line: "#",
	block: null,
	quotes: ['"', "'"],
	raw: null,
	keys: true,
};

const JSONL = {
	keywords: words(""),
	constants: words("true false null"),
	types: words(""),
	builtins: words(""),
	line: null,
	block: null,
	quotes: ['"'],
	raw: null,
	keys: true,
};

const LANGS = { tin: TIN, go: GO, js: JS, c: C, shell: SHELL, python: PY, sql: SQL, css: CSS, yaml: YAML, json: JSONL };

const isIdStart = (c) => /[A-Za-z_À-￿]/.test(c);
const isId = (c) => /[A-Za-z0-9_À-￿]/.test(c);
const isDigit = (c) => c >= "0" && c <= "9";

// lines tokenizes text: an array, one entry per line, of [cls, text] pairs ("" for plain text).
export function lines(text, lang) {
	const src = String(text).replace(/\r\n?/g, "\n");
	const rows = src.split("\n");
	if (rows.length && rows[rows.length - 1] === "" && src.endsWith("\n")) rows.pop();
	const def = LANGS[lang];
	if (lang === "markdown") return rows.map(markdownLine);
	if (lang === "html") return rows.map(htmlLine);
	if (!def) return rows.map((r) => (r ? [["", r]] : []));
	const state = { inBlock: false, inRaw: false, inStr: "" };
	return rows.map((r) => tokenizeLine(r, def, state));
}

function tokenizeLine(line, def, st) {
	const out = [];
	const push = (cls, v) => {
		if (!v) return;
		const last = out[out.length - 1];
		if (last && last[0] === cls) last[1] += v;
		else out.push([cls, v]);
	};
	let i = 0;
	const n = line.length;
	if (st.inBlock) {
		const end = line.indexOf(def.block[1]);
		if (end < 0) {
			push("tok-com", line);
			return out;
		}
		push("tok-com", line.slice(0, end + def.block[1].length));
		i = end + def.block[1].length;
		st.inBlock = false;
	}
	if (st.inRaw) {
		const end = line.indexOf(def.raw);
		if (end < 0) {
			push("tok-str", line);
			return out;
		}
		push("tok-str", line.slice(0, end + 1));
		i = end + 1;
		st.inRaw = false;
	}
	if (def.preproc && /^\s*#/.test(line)) {
		push("tok-attr", line);
		return out;
	}
	let prevWord = "";
	while (i < n) {
		const c = line[i];
		if (def.line && line.startsWith(def.line, i)) {
			push("tok-com", line.slice(i));
			break;
		}
		if (def.block && line.startsWith(def.block[0], i)) {
			const end = line.indexOf(def.block[1], i + def.block[0].length);
			if (end < 0) {
				push("tok-com", line.slice(i));
				st.inBlock = true;
				break;
			}
			push("tok-com", line.slice(i, end + def.block[1].length));
			i = end + def.block[1].length;
			continue;
		}
		if (def.raw && c === def.raw) {
			const end = line.indexOf(def.raw, i + 1);
			if (end < 0) {
				push("tok-str", line.slice(i));
				st.inRaw = true;
				break;
			}
			push("tok-str", line.slice(i, end + 1));
			i = end + 1;
			continue;
		}
		if (def.quotes.includes(c)) {
			let j = i + 1;
			const isKey = def.keys;
			let segStart = i;
			const parts = [];
			while (j < n && line[j] !== c) {
				if (line[j] === "\\") {
					j += 2;
					continue;
				}
				if (def.interp && c === '"' && line[j] === "{") {
					if (line[j + 1] === "{") {
						j += 2;
						continue;
					}
					// an interpolation: {expr}
					parts.push(["tok-str", line.slice(segStart, j)]);
					let depth = 1;
					let k = j + 1;
					while (k < n && depth > 0) {
						if (line[k] === "{") depth++;
						else if (line[k] === "}") depth--;
						if (depth > 0) k++;
					}
					parts.push(["tok-interp", line.slice(j, Math.min(k + 1, n))]);
					j = k + 1;
					segStart = j;
					continue;
				}
				j++;
			}
			const end = Math.min(j + 1, n);
			parts.push(["tok-str", line.slice(segStart, end)]);
			let rest = end;
			while (rest < n && line[rest] === " ") rest++;
			const keyLike = isKey && line[rest] === ":";
			for (const [cls, v] of parts) push(keyLike ? "tok-fn" : cls, v);
			i = end;
			continue;
		}
		if (def.vars && c === "$") {
			const m = /^\$(\{[^}]*\}|[A-Za-z_][A-Za-z0-9_]*|[0-9@#?*$!-])/.exec(line.slice(i));
			if (m) {
				push("tok-const", m[0]);
				i += m[0].length;
				continue;
			}
		}
		if (def.attrs && c === "@" && isIdStart(line[i + 1] || "")) {
			let j = i + 1;
			while (j < n && isId(line[j])) j++;
			push("tok-attr", line.slice(i, j));
			i = j;
			continue;
		}
		if (isDigit(c) || (c === "." && isDigit(line[i + 1] || "") && !isId(line[i - 1] || ""))) {
			const m = /^(0[xX][0-9a-fA-F_]+|0[oO][0-7_]+|0[bB][01_]+|(\d[\d_]*)?\.?\d[\d_]*([eE][+-]?\d+)?)/.exec(line.slice(i));
			let len = m ? m[0].length : 1;
			if (def.units) {
				const u = /^(ns|us|ms|s|m|h|b|kb|mb|gb)(?![A-Za-z0-9_])/.exec(line.slice(i + len));
				if (u) len += u[0].length;
			}
			push("tok-num", line.slice(i, i + len));
			i += len;
			continue;
		}
		if (isIdStart(c)) {
			let j = i + 1;
			while (j < n && isId(line[j])) j++;
			const w = line.slice(i, j);
			const lw = def.ci ? w.toLowerCase() : w;
			let k = j;
			while (k < n && line[k] === " ") k++;
			let cls = "";
			if (def.keywords.has(lw)) cls = "tok-kw";
			else if (def.constants.has(lw)) cls = "tok-const";
			else if (def.types.has(lw)) cls = "tok-ty";
			else if (def.keys && line[k] === ":") cls = "tok-fn";
			else if (prevWord === "fn" || prevWord === "func" || prevWord === "function" || prevWord === "def") cls = "tok-fn";
			else if (prevWord === "type" || prevWord === "struct" || prevWord === "enum" || prevWord === "class" || prevWord === "shape") cls = "tok-ty";
			else if (line[k] === "(" || (line[j] === "[" && def === TIN && /^\[[A-Z]/.test(line.slice(j)))) cls = def.builtins.has(lw) ? "tok-kw" : "tok-fn";
			else if (/^[A-Z][A-Z0-9_]+$/.test(w) && w.length > 1) cls = "tok-const";
			else if (/^[A-Z]/.test(w) && def !== SQL && def !== SHELL && (def !== TIN || line[k] === "{" || prevWord === ":")) cls = "tok-ty";
			push(cls, w);
			prevWord = lw;
			i = j;
			continue;
		}
		if (c === " " || c === "\t") {
			let j = i + 1;
			while (j < n && (line[j] === " " || line[j] === "\t")) j++;
			push("", line.slice(i, j));
			i = j;
			continue;
		}
		if ("+-*/%&|^<>=!?~:.".includes(c)) {
			push("tok-op", c);
			if (c !== ".") prevWord = "";
			i++;
			continue;
		}
		push("", c);
		prevWord = "";
		i++;
	}
	return out;
}

function markdownLine(line) {
	if (/^\s*#{1,6}\s/.test(line)) return [["tok-kw", line]];
	if (/^\s*(```|~~~)/.test(line)) return [["tok-com", line]];
	if (/^\s*>/.test(line)) return [["tok-com", line]];
	const out = [];
	const m = /^(\s*(?:[-*+]|\d+[.)])\s+)/.exec(line);
	let rest = line;
	if (m) {
		out.push(["tok-op", m[1]]);
		rest = line.slice(m[1].length);
	}
	const re = /(`[^`]+`|\[[^\]]*\]\([^)]*\)|\*\*[^*]+\*\*)/g;
	let last = 0;
	let x;
	while ((x = re.exec(rest))) {
		if (x.index > last) out.push(["", rest.slice(last, x.index)]);
		out.push([x[0][0] === "`" ? "tok-str" : x[0][0] === "[" ? "tok-fn" : "tok-kw", x[0]]);
		last = x.index + x[0].length;
	}
	if (last < rest.length) out.push(["", rest.slice(last)]);
	return out;
}

function htmlLine(line) {
	const out = [];
	const re = /(<!--.*?-->|<\/?[A-Za-z][^>]*>?)/g;
	let last = 0;
	let x;
	while ((x = re.exec(line))) {
		if (x.index > last) out.push(["", line.slice(last, x.index)]);
		if (x[0].startsWith("<!--")) out.push(["tok-com", x[0]]);
		else {
			const t = /^(<\/?)([A-Za-z][\w-]*)(.*?)(\/?>?)$/.exec(x[0]);
			if (t) {
				out.push(["tok-op", t[1]], ["tok-kw", t[2]]);
				const attrRe = /([\w:-]+)(=)("[^"]*"|'[^']*'|[^\s>]+)?/g;
				let al = 0;
				let a;
				while ((a = attrRe.exec(t[3]))) {
					if (a.index > al) out.push(["", t[3].slice(al, a.index)]);
					out.push(["tok-attr", a[1]], ["tok-op", a[2]], ["tok-str", a[3] || ""]);
					al = a.index + a[0].length;
				}
				if (al < t[3].length) out.push(["", t[3].slice(al)]);
				out.push(["tok-op", t[4]]);
			} else out.push(["", x[0]]);
		}
		last = x.index + x[0].length;
	}
	if (last < line.length) out.push(["", line.slice(last)]);
	return out;
}

// outline lists a Tin file's top-level declarations: {kind, name, recv, line} (1-based lines).
export function outline(text) {
	const out = [];
	const rows = String(text).split("\n");
	let inRaw = false;
	rows.forEach((row, i) => {
		const ticks = (row.match(/`/g) || []).length;
		const wasRaw = inRaw;
		if (ticks % 2 === 1) inRaw = !inRaw;
		if (wasRaw) return;
		let m;
		if ((m = /^fn\s+\(\s*\w+\s+(?:mut\s+)?\*?(\w+)[^)]*\)\s*(\w+)/.exec(row))) out.push({ kind: "method", name: m[2], recv: m[1], line: i + 1 });
		else if ((m = /^fn\s+(\w+)/.exec(row))) out.push({ kind: "fn", name: m[1], recv: "", line: i + 1 });
		else if ((m = /^type\s+(\w+)(?:\[[^\]]*\])?\s+(shape|enum|struct)?/.exec(row))) out.push({ kind: m[2] === "shape" ? "shape" : "type", name: m[1], recv: "", line: i + 1 });
		else if ((m = /^(const|let|mut)\s+(\w+)/.exec(row))) out.push({ kind: m[1], name: m[2], recv: "", line: i + 1 });
		else if ((m = /^(test|bench)\s+"([^"]*)"/.exec(row))) out.push({ kind: m[1], name: m[2], recv: "", line: i + 1 });
	});
	return out;
}
