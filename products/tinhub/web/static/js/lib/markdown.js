// A safe Markdown reader (CommonMark's common subset, with reference links and entities, plus GitHub tables, task
// lists, strikethrough and bare links).
// parse makes a tree of plain objects with no DOM, so it is tested in Node (web/test/markdown.test.js); render builds
// DOM from it with text nodes only. Raw HTML is shown as text, and only http(s), mailto and relative links are kept.

// safeUrl is url when it is http(s), mailto, an anchor or relative; else "".
export function safeUrl(url) {
	const u = String(url || "").trim();
	if (u === "") return "";
	if (/^(https?:|mailto:)/i.test(u)) return u;
	if (/^[a-z][a-z0-9+.-]*:/i.test(u)) return "";
	if (u.startsWith("//")) return "";
	return u;
}

// slug is a heading's anchor id, as GitHub makes it.
export function slug(text) {
	return String(text)
		.toLowerCase()
		.trim()
		.replace(/[^\p{L}\p{N}\s_-]/gu, "")
		.replace(/\s/g, "-");
}

const FENCE = /^ {0,3}(`{3,}|~{3,})\s*([^`\s]*)[^`]*$/;
const ATX = /^ {0,3}(#{1,6})(?:[ \t]+(.*?))?(?:[ \t]+#+)?[ \t]*$/;
const HR = /^ {0,3}((\*[ \t]*){3,}|(-[ \t]*){3,}|(_[ \t]*){3,})$/;
const BULLET = /^( {0,3})([-*+])([ \t]+|$)(.*)$/;
const ORDERED = /^( {0,3})(\d{1,9})([.)])([ \t]+|$)(.*)$/;
const QUOTE = /^ {0,3}> ?(.*)$/;
const HTML_BLOCK = /^ {0,3}(<!--|<\/?(p|div|img|picture|details|summary|table|center|h[1-6]|br|hr|a|ul|ol|li|dl|blockquote|pre|figure|section|kbd|sub|sup|b|strong|em|i|span|source)(\s|\/?>|$))/i;
const INLINE_TAG = /^<(\/?)(br|img|kbd|sub|sup|b|strong|i|em|u|s|del|ins|code|span|a|mark|small|abbr)(\s[^<>]*)?\/?>/i;
const TABLE_SEP = /^ {0,3}\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$/;

function isBlank(l) {
	return /^\s*$/.test(l);
}

function splitRow(line) {
	let s = line.trim();
	if (s.startsWith("|")) s = s.slice(1);
	if (s.endsWith("|") && !s.endsWith("\\|")) s = s.slice(0, -1);
	const cells = [];
	let cur = "";
	let code = false;
	for (let i = 0; i < s.length; i++) {
		const c = s[i];
		if (c === "\\" && s[i + 1] === "|") {
			cur += "|";
			i++;
		} else if (c === "`") {
			code = !code;
			cur += c;
		} else if (c === "|" && !code) {
			cells.push(cur.trim());
			cur = "";
		} else cur += c;
	}
	cells.push(cur.trim());
	return cells;
}

// parse reads Markdown source into a list of blocks.
export function parse(src) {
	const lines = String(src || "").replace(/\r\n?/g, "\n").replace(/\t/g, "    ").split("\n");
	const saved = refs;
	refs = definitions(lines);
	try {
		return blocks(lines);
	} finally {
		refs = saved;
	}
}

// refs maps the link reference definitions of the source being parsed ("[label]: url 'title'"), by normalized label.
let refs = new Map();

const DEF = /^ {0,3}\[([^\]]+)\]:[ \t]*<?([^\s<>]+)>?(?:[ \t]+(?:"([^"]*)"|'([^']*)'|\(([^)]*)\)))?[ \t]*$/;

const refLabel = (l) => l.trim().replace(/\s+/g, " ").toLowerCase();

// definitions takes the reference definitions out of lines (blanking them, outside code fences, where a paragraph
// could start) and returns them; the first definition of a label wins.
function definitions(lines) {
	const out = new Map();
	let fence = "";
	let prevBlank = true;
	for (let i = 0; i < lines.length; i++) {
		const l = lines[i];
		const f = FENCE.exec(l);
		if (fence) {
			if (f && f[1][0] === fence[0] && f[1].length >= fence.length && !f[2]) {
				fence = "";
				prevBlank = true;
				continue;
			}
		} else if (f) fence = f[1];
		else if (prevBlank) {
			const m = DEF.exec(l);
			if (m && /\S/.test(m[1])) {
				const label = refLabel(m[1]);
				if (!out.has(label)) out.set(label, { href: m[2], title: m[3] ?? m[4] ?? m[5] ?? "" });
				lines[i] = "";
				prevBlank = true;
				continue;
			}
		}
		prevBlank = isBlank(l) || ATX.test(l) || HR.test(l);
	}
	return out;
}

// ENTITIES are the named character references read in text; others stay as written.
const ENTITIES = { amp: "&", lt: "<", gt: ">", quot: '"', apos: "'", nbsp: "\u00a0", ensp: "\u2002", emsp: "\u2003", thinsp: "\u2009", copy: "©", reg: "®", trade: "™", hellip: "…", mdash: "—", ndash: "–", lsquo: "‘", rsquo: "’", ldquo: "“", rdquo: "”", laquo: "«", raquo: "»", middot: "·", bull: "•", deg: "°", plusmn: "±", times: "×", divide: "÷", para: "¶", sect: "§", cent: "¢", pound: "£", euro: "€", yen: "¥", larr: "←", rarr: "→", uarr: "↑", darr: "↓", harr: "↔", lArr: "⇐", rArr: "⇒", hArr: "⇔", le: "≤", ge: "≥", ne: "≠", infin: "∞", check: "✓", star: "☆", hearts: "♥", zwj: "\u200d", zwnj: "\u200c", shy: "\u00ad" };

// entities reads the character references in text: &copy;, &#169; and &#xA9;.
export function entities(text) {
	return text.replace(/&(?:#(\d{1,7})|#[xX]([0-9a-fA-F]{1,6})|([A-Za-z][A-Za-z0-9]{1,31}));/g, (all, dec, hex, name) => {
		if (name) return Object.hasOwn(ENTITIES, name) ? ENTITIES[name] : all;
		const n = dec ? parseInt(dec, 10) : parseInt(hex, 16);
		return n === 0 || n > 0x10ffff || (n >= 0xd800 && n <= 0xdfff) ? "\ufffd" : String.fromCodePoint(n);
	});
}

function blocks(lines) {
	const out = [];
	let i = 0;
	while (i < lines.length) {
		const line = lines[i];
		if (isBlank(line)) {
			i++;
			continue;
		}
		let m;
		if ((m = FENCE.exec(line))) {
			const fence = m[1];
			const lang = m[2] || "";
			const indent = line.length - line.trimStart().length;
			const body = [];
			i++;
			while (i < lines.length) {
				const l = lines[i];
				const close = new RegExp("^ {0,3}" + fence[0] + "{" + fence.length + ",}\\s*$");
				if (close.test(l)) {
					i++;
					break;
				}
				body.push(l.slice(Math.min(indent, l.length - l.trimStart().length)));
				i++;
			}
			out.push({ t: "code", lang, v: body.join("\n") });
			continue;
		}
		if ((m = ATX.exec(line))) {
			out.push({ t: "h", level: m[1].length, c: inline(m[2] || "") });
			i++;
			continue;
		}
		if (HR.test(line)) {
			out.push({ t: "hr" });
			i++;
			continue;
		}
		if (/^ {4,}/.test(line)) {
			const body = [];
			while (i < lines.length && (/^ {4,}/.test(lines[i]) || (isBlank(lines[i]) && i + 1 < lines.length && /^ {4,}/.test(lines[i + 1])))) {
				body.push(lines[i].slice(4));
				i++;
			}
			out.push({ t: "code", lang: "", v: body.join("\n") });
			continue;
		}
		if (QUOTE.test(line)) {
			const body = [];
			while (i < lines.length && !isBlank(lines[i])) {
				const q = QUOTE.exec(lines[i]);
				body.push(q ? q[1] : lines[i]);
				i++;
			}
			out.push({ t: "quote", c: blocks(body) });
			continue;
		}
		if (BULLET.test(line) || ORDERED.test(line)) {
			const res = list(lines, i);
			out.push(res.node);
			i = res.next;
			continue;
		}
		if (line.includes("|") && i + 1 < lines.length && TABLE_SEP.test(lines[i + 1]) && lines[i + 1].includes("-")) {
			const head = splitRow(line);
			const aligns = splitRow(lines[i + 1]).map((c) => (c.startsWith(":") && c.endsWith(":") ? "center" : c.endsWith(":") ? "right" : c.startsWith(":") ? "left" : ""));
			i += 2;
			const rows = [];
			while (i < lines.length && !isBlank(lines[i]) && lines[i].includes("|")) {
				rows.push(splitRow(lines[i]).map(inline));
				i++;
			}
			out.push({ t: "table", aligns, head: head.map(inline), rows });
			continue;
		}
		if (HTML_BLOCK.test(line)) {
			const body = [];
			while (i < lines.length && !isBlank(lines[i])) body.push(lines[i++]);
			out.push({ t: "html", v: body.join("\n") });
			continue;
		}
		// a paragraph, or a setext heading
		const para = [line.trimStart()];
		i++;
		let level = 0;
		while (i < lines.length && !isBlank(lines[i])) {
			const l = lines[i];
			if (/^ {0,3}=+\s*$/.test(l)) {
				level = 1;
				i++;
				break;
			}
			if (/^ {0,3}-+\s*$/.test(l) && para.length > 0) {
				level = 2;
				i++;
				break;
			}
			if (FENCE.test(l) || ATX.test(l) || HR.test(l) || QUOTE.test(l) || BULLET.test(l) || /^( {0,3})1[.)]\s/.test(l)) break;
			para.push(l.trimStart());
			i++;
		}
		const textSrc = para.join("\n").trimEnd();
		out.push(level ? { t: "h", level, c: inline(textSrc) } : { t: "p", c: inline(textSrc) });
	}
	return out;
}

function list(lines, start) {
	const first = BULLET.exec(lines[start]) || ORDERED.exec(lines[start]);
	const ordered = !BULLET.exec(lines[start]);
	const node = { t: "list", ordered, start: ordered ? Number(first[2]) : 1, items: [], loose: false };
	let i = start;
	while (i < lines.length) {
		const m = ordered ? ORDERED.exec(lines[i]) : BULLET.exec(lines[i]);
		if (!m) break;
		if (!ordered && m[2] !== first[2]) break;
		const lead = m[1].length;
		const content = ordered ? m[5] : m[4];
		const marker = ordered ? m[2].length + 1 : 1;
		const pad = ordered ? m[4].length : m[3].length;
		const indent = lead + marker + Math.max(1, Math.min(pad, 4));
		const body = [content];
		i++;
		let sawBlank = false;
		while (i < lines.length) {
			const l = lines[i];
			if (isBlank(l)) {
				body.push("");
				sawBlank = true;
				i++;
				continue;
			}
			const ind = l.length - l.trimStart().length;
			if (ind >= indent) {
				body.push(l.slice(indent));
				i++;
				continue;
			}
			if (!sawBlank && !BULLET.test(l) && !ORDERED.test(l) && !FENCE.test(l) && !ATX.test(l) && !HR.test(l) && !QUOTE.test(l)) {
				body.push(l.trim());
				i++;
				continue;
			}
			break;
		}
		while (body.length && body[body.length - 1] === "") {
			body.pop();
			if (i < lines.length && (BULLET.test(lines[i]) || ORDERED.test(lines[i]))) node.loose = true;
		}
		let task = null;
		const tm = /^\[([ xX])\][ \t]+/.exec(body[0]);
		if (tm) {
			task = tm[1] !== " ";
			body[0] = body[0].slice(tm[0].length);
		}
		const inner = blocks(body);
		if (body.includes("")) node.loose = true;
		node.items.push({ task, c: inner });
	}
	return { node, next: i };
}

const PUNCT = /[!-/:-@[-`{-~]/;

// inline reads a span of inline Markdown.
export function inline(src) {
	const out = [];
	let buf = "";
	const flush = () => {
		if (buf) {
			out.push({ t: "text", v: entities(buf) });
			buf = "";
		}
	};
	let i = 0;
	const s = String(src);
	while (i < s.length) {
		const c = s[i];
		if (c === "\\" && i + 1 < s.length && PUNCT.test(s[i + 1])) {
			buf += s[i + 1];
			i += 2;
			continue;
		}
		if (c === "\\" && s[i + 1] === "\n") {
			flush();
			out.push({ t: "br" });
			i += 2;
			continue;
		}
		if (c === "\n") {
			if (buf.endsWith("  ")) {
				buf = buf.replace(/ +$/, "");
				flush();
				out.push({ t: "br" });
			} else buf += " ";
			i++;
			continue;
		}
		if (c === "`") {
			let n = 0;
			while (s[i + n] === "`") n++;
			const ticks = "`".repeat(n);
			const end = s.indexOf(ticks, i + n);
			if (end > 0 && s[end + n] !== "`") {
				flush();
				let code = s.slice(i + n, end).replace(/\n/g, " ");
				if (code.length > 2 && code.startsWith(" ") && code.endsWith(" ") && code.trim()) code = code.slice(1, -1);
				out.push({ t: "code", v: code });
				i = end + n;
				continue;
			}
			buf += ticks;
			i += n;
			continue;
		}
		if (c === "!" && s[i + 1] === "[") {
			const l = link(s, i + 1) || refLink(s, i + 1);
			if (l) {
				flush();
				const src2 = safeUrl(l.href);
				out.push({ t: "img", src: src2, alt: plain(inline(l.text)), title: l.title });
				i = l.end;
				continue;
			}
		}
		if (c === "[") {
			const l = link(s, i) || refLink(s, i);
			if (l) {
				flush();
				out.push({ t: "link", href: safeUrl(l.href), title: l.title, c: inline(l.text) });
				i = l.end;
				continue;
			}
		}
		if (c === "<" && s[i + 1] === "!" && s.startsWith("<!--", i)) {
			const end = s.indexOf("-->", i + 4);
			if (end > 0) {
				flush();
				i = end + 3;
				continue;
			}
		}
		if (c === "<") {
			const t = INLINE_TAG.exec(s.slice(i));
			if (t) {
				flush();
				out.push({ t: "tag", v: t[0] });
				i += t[0].length;
				continue;
			}
		}
		if (c === "<") {
			const m = /^<((?:https?:\/\/|mailto:)[^\s<>]+)>/i.exec(s.slice(i));
			if (m) {
				flush();
				out.push({ t: "link", href: safeUrl(m[1]), title: "", c: [{ t: "text", v: m[1].replace(/^mailto:/i, "") }] });
				i += m[0].length;
				continue;
			}
		}
		if ((c === "h" || c === "w") && (i === 0 || /[\s(]/.test(s[i - 1]))) {
			const m = /^(https?:\/\/|www\.)[^\s<]*[^\s<?!.,:*_~)'"]/i.exec(s.slice(i));
			if (m) {
				flush();
				const href = m[0].startsWith("www.") ? "https://" + m[0] : m[0];
				out.push({ t: "link", href: safeUrl(href), title: "", c: [{ t: "text", v: m[0] }] });
				i += m[0].length;
				continue;
			}
		}
		if (c === "*" || c === "_" || c === "~") {
			let n = 0;
			while (s[i + n] === c) n++;
			if (c === "~" && n !== 2) {
				buf += s.slice(i, i + n);
				i += n;
				continue;
			}
			const prev = i > 0 ? s[i - 1] : " ";
			const next = s[i + n] || " ";
			const canOpen = !/\s/.test(next) && !(c === "_" && /[\p{L}\p{N}]/u.test(prev));
			if (canOpen) {
				const take = c === "~" ? 2 : n >= 2 ? 2 : 1;
				const close = findClose(s, i + take, c, take);
				if (close > 0) {
					flush();
					const inner = inline(s.slice(i + take, close));
					const t = c === "~" ? "del" : take === 2 ? "strong" : "em";
					if (n === 3 && c !== "~") {
						// ***x*** is strong inside em
						const close3 = findClose(s, i + 3, c, 3);
						if (close3 > 0) {
							out.push({ t: "em", c: [{ t: "strong", c: inline(s.slice(i + 3, close3)) }] });
							i = close3 + 3;
							continue;
						}
					}
					out.push({ t, c: inner });
					i = close + take;
					continue;
				}
			}
			buf += s.slice(i, i + n);
			i += n;
			continue;
		}
		buf += c;
		i++;
	}
	flush();
	return out;
}

function findClose(s, from, c, n) {
	const run = c.repeat(n);
	let i = from;
	while (i < s.length) {
		if (s[i] === "\\") {
			i += 2;
			continue;
		}
		if (s[i] === "`") {
			const end = s.indexOf("`", i + 1);
			if (end > 0) {
				i = end + 1;
				continue;
			}
		}
		if (s.startsWith(run, i) && i > from && !/\s/.test(s[i - 1])) {
			let k = 0;
			while (s[i + k] === c) k++;
			if (k === n || (k > n && n === 1 && k !== 2) || (n === 2 && k >= 2) || (n === 3 && k >= 3)) {
				const after = s[i + k] || " ";
				if (c !== "_" || !/[\p{L}\p{N}]/u.test(after)) return i;
			}
			i += k;
			continue;
		}
		i++;
	}
	return -1;
}

function link(s, i) {
	let depth = 0;
	let j = i;
	for (; j < s.length; j++) {
		if (s[j] === "\\") {
			j++;
			continue;
		}
		if (s[j] === "[") depth++;
		else if (s[j] === "]") {
			depth--;
			if (depth === 0) break;
		}
	}
	if (j >= s.length || s[j + 1] !== "(") return null;
	const text = s.slice(i + 1, j);
	let k = j + 2;
	while (s[k] === " ") k++;
	let href = "";
	if (s[k] === "<") {
		const end = s.indexOf(">", k);
		if (end < 0) return null;
		href = s.slice(k + 1, end);
		k = end + 1;
	} else {
		let paren = 0;
		const st = k;
		for (; k < s.length; k++) {
			const ch = s[k];
			if (ch === "\\") {
				k++;
				continue;
			}
			if (ch === "(") paren++;
			else if (ch === ")") {
				if (paren === 0) break;
				paren--;
			} else if (ch === " ") break;
		}
		href = s.slice(st, k);
	}
	while (s[k] === " ") k++;
	let title = "";
	if (s[k] === '"' || s[k] === "'") {
		const q = s[k];
		const end = s.indexOf(q, k + 1);
		if (end < 0) return null;
		title = s.slice(k + 1, end);
		k = end + 1;
		while (s[k] === " ") k++;
	}
	if (s[k] !== ")") return null;
	return { text, href: entities(href), title: entities(title), end: k + 1 };
}

// closeBracket is the index of the "]" that closes the "[" at i, or -1.
function closeBracket(s, i) {
	let depth = 0;
	for (let j = i; j < s.length; j++) {
		if (s[j] === "\\") j++;
		else if (s[j] === "[") depth++;
		else if (s[j] === "]" && --depth === 0) return j;
	}
	return -1;
}

// refLink reads a reference link at i ("[text][label]", "[label][]" or "[label]") whose label is defined.
function refLink(s, i) {
	if (!refs.size) return null;
	const j = closeBracket(s, i);
	if (j < 0) return null;
	const text = s.slice(i + 1, j);
	let label = text;
	let end = j + 1;
	if (s[j + 1] === "[") {
		const k = closeBracket(s, j + 1);
		if (k < 0) return null;
		if (k > j + 2) label = s.slice(j + 2, k);
		end = k + 1;
	}
	const def = refs.get(refLabel(label));
	if (!def) return null;
	return { text, href: entities(def.href), title: entities(def.title), end };
}

// plain is a node list's text, without markup.
export function plain(nodes) {
	let out = "";
	for (const n of nodes) {
		if (n.t === "text" || n.t === "code") out += n.v;
		else if (n.t === "br") out += " ";
		else if (n.c) out += plain(n.c);
		else if (n.t === "img") out += n.alt;
		else if (n.t === "tag") out += "";
	}
	return out;
}

// render builds DOM from parsed blocks with h (lib/dom.js). opts.link(href) rewrites relative links and
// opts.image(src) relative images; opts.code(lang, text) may return a highlighted element.
export function render(nodes, h, opts = {}) {
	const usedSlugs = new Map();
	const inl = (list) => (opts.html && list.some((n) => n.t === "tag") ? opts.html(toHtml(list)) : list.map((n) => renderInline(n, h, opts)));
	const blk = (list, tight) =>
		list.map((b) => {
			switch (b.t) {
				case "p":
					return tight ? inl(b.c) : h("p", {}, inl(b.c));
				case "h": {
					let id = slug(plain(b.c));
					const n = usedSlugs.get(id) || 0;
					usedSlugs.set(id, n + 1);
					if (n) id += "-" + n;
					return h("h" + b.level, { id: "user-content-" + id }, inl(b.c));
				}
				case "code": {
					const code = opts.code ? opts.code(b.lang, b.v) : null;
					return h("pre", {}, code || h("code", {}, b.v));
				}
				case "quote":
					return h("blockquote", {}, blk(b.c));
				case "hr":
					return h("hr");
				case "html":
					return opts.html ? opts.html(b.v) : h("p", {}, b.v);
				case "list":
					return h(
						b.ordered ? "ol" : "ul",
						b.ordered && b.start !== 1 ? { start: b.start } : {},
						b.items.map((it) =>
							h(
								"li",
								{ class: it.task === null ? "" : "task" },
								it.task === null ? null : h("input", { type: "checkbox", checked: it.task, disabled: true }),
								blk(it.c, !b.loose),
							),
						),
					);
				case "table":
					return h(
						"table",
						{},
						h("thead", {}, h("tr", {}, b.head.map((c, i) => h("th", { style: b.aligns[i] ? { "text-align": b.aligns[i] } : null }, inl(c))))),
						h("tbody", {}, b.rows.map((r) => h("tr", {}, b.head.map((_, i) => h("td", { style: b.aligns[i] ? { "text-align": b.aligns[i] } : null }, inl(r[i] || [])))))),
					);
				default:
					return null;
			}
		});
	return blk(nodes);
}

const esc = (t) => String(t).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

// toHtml serializes inline nodes (text escaped) so that raw tags among them can be sanitized together.
export function toHtml(list) {
	return list
		.map((n) => {
			switch (n.t) {
				case "text":
					return esc(n.v);
				case "tag":
					return n.v;
				case "code":
					return `<code>${esc(n.v)}</code>`;
				case "br":
					return "<br>";
				case "em":
				case "strong":
				case "del":
					return `<${n.t}>${toHtml(n.c)}</${n.t}>`;
				case "link":
					return n.href ? `<a href="${esc(n.href)}"${n.title ? ` title="${esc(n.title)}"` : ""}>${toHtml(n.c)}</a>` : toHtml(n.c);
				case "img":
					return n.src ? `<img src="${esc(n.src)}" alt="${esc(n.alt)}">` : esc(n.alt);
				default:
					return "";
			}
		})
		.join("");
}

function renderInline(n, h, opts) {
	switch (n.t) {
		case "text":
			return n.v;
		case "code":
			return h("code", {}, n.v);
		case "br":
			return h("br");
		case "em":
			return h("em", {}, n.c.map((x) => renderInline(x, h, opts)));
		case "strong":
			return h("strong", {}, n.c.map((x) => renderInline(x, h, opts)));
		case "del":
			return h("del", {}, n.c.map((x) => renderInline(x, h, opts)));
		case "link": {
			const kids = n.c.map((x) => renderInline(x, h, opts));
			if (!n.href) return h("span", {}, kids);
			const href = opts.link ? opts.link(n.href) : n.href;
			const external = /^https?:/i.test(href);
			return h("a", { href, title: n.title || null, rel: external ? "nofollow noopener noreferrer" : null, target: external ? "_blank" : null }, kids);
		}
		case "tag":
			return n.v;
		case "img": {
			if (!n.src) return n.alt;
			const src = opts.image ? opts.image(n.src) : n.src;
			if (!src) return n.alt;
			return h("img", { src, alt: n.alt, title: n.title || null, loading: "lazy" });
		}
		default:
			return null;
	}
}
