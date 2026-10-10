// Formatting helpers with no DOM: times, sizes, ids and names. Tested by web/test/format.test.js.

// toMs reads a timestamp the API sent in seconds, milliseconds or microseconds as milliseconds.
export function toMs(t) {
	if (t === null || t === undefined || t === "" || t === 0) return 0;
	if (typeof t === "string") {
		const n = Number(t);
		if (!Number.isNaN(n)) return toMs(n);
		const d = Date.parse(t);
		return Number.isNaN(d) ? 0 : d;
	}
	if (t > 1e14) return Math.floor(t / 1000);
	if (t > 1e11) return t;
	return t * 1000;
}

const UNITS = [
	["year", 365 * 24 * 3600],
	["month", 30 * 24 * 3600],
	["week", 7 * 24 * 3600],
	["day", 24 * 3600],
	["hour", 3600],
	["minute", 60],
];

// ago is a time as "3 hours ago", "just now", or "in 2 days".
export function ago(t, now = Date.now()) {
	const ms = toMs(t);
	if (!ms) return "never";
	const secs = Math.round((now - ms) / 1000);
	const abs = Math.abs(secs);
	if (abs < 45) return "just now";
	for (const [name, size] of UNITS) {
		if (abs >= size) {
			const n = Math.round(abs / size);
			const unit = n === 1 ? name : name + "s";
			return secs >= 0 ? `${n} ${unit} ago` : `in ${n} ${unit}`;
		}
	}
	const n = Math.max(1, Math.round(abs / 60));
	return secs >= 0 ? `${n} minute${n === 1 ? "" : "s"} ago` : `in ${n} minute${n === 1 ? "" : "s"}`;
}

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

// date is "Oct 9, 2026"; with time, "Oct 9, 2026, 14:05".
export function date(t, withTime = false) {
	const ms = toMs(t);
	if (!ms) return "";
	const d = new Date(ms);
	let out = `${MONTHS[d.getMonth()]} ${d.getDate()}, ${d.getFullYear()}`;
	if (withTime) out += `, ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
	return out;
}

// iso is the time as an ISO 8601 string, for title attributes.
export function iso(t) {
	const ms = toMs(t);
	return ms ? new Date(ms).toISOString() : "";
}

// bytes is a size as "1.4 MB".
export function bytes(n) {
	if (n === null || n === undefined || Number.isNaN(n)) return "";
	if (n < 1024) return `${n} B`;
	const units = ["KB", "MB", "GB", "TB"];
	let v = n / 1024;
	let i = 0;
	while (v >= 1024 && i < units.length - 1) {
		v /= 1024;
		i++;
	}
	return `${v >= 100 ? Math.round(v) : v.toFixed(1).replace(/\.0$/, "")} ${units[i]}`;
}

// count is a number with thousands separators: 12,345.
export function count(n) {
	return Number(n || 0).toLocaleString("en-US");
}

// plural is "1 change" or "3 changes".
export function plural(n, one, many = one + "s") {
	return `${count(n)} ${n === 1 ? one : many}`;
}

// short is the first 10 characters of a commit or object id.
export function short(id, n = 10) {
	return id ? String(id).slice(0, n) : "";
}

// shortChange is a change id's first 12 letters, the length tit prints.
export function shortChange(id) {
	return id ? String(id).slice(0, 12) : "";
}

// title is a commit message's first line; body the rest, trimmed.
export function title(msg) {
	if (!msg) return "";
	const i = msg.indexOf("\n");
	return (i < 0 ? msg : msg.slice(0, i)).trim();
}

export function body(msg) {
	if (!msg) return "";
	const i = msg.indexOf("\n");
	return i < 0 ? "" : msg.slice(i + 1).trim();
}

// hue is a stable hue in [0, 360) for a name, for avatars and chart series.
export function hue(name) {
	let h = 2166136261;
	for (const ch of String(name || "")) {
		h ^= ch.codePointAt(0);
		h = Math.imul(h, 16777619);
	}
	return Math.abs(h) % 360;
}

// initials are one or two letters for an avatar: "Ada Lovelace" is AL, "ada" is A.
export function initials(name) {
	const words = String(name || "?").trim().split(/[\s._-]+/).filter(Boolean);
	if (words.length === 0) return "?";
	if (words.length === 1) return [...words[0]][0].toUpperCase();
	return ([...words[0]][0] + [...words[words.length - 1]][0]).toUpperCase();
}

// duration is a span of milliseconds as "1.2s", "340ms" or "3m 12s".
export function duration(ms) {
	if (ms === null || ms === undefined) return "";
	if (ms < 1000) return `${Math.round(ms)}ms`;
	if (ms < 60000) return `${(ms / 1000).toFixed(1).replace(/\.0$/, "")}s`;
	const m = Math.floor(ms / 60000);
	const sec = Math.round((ms % 60000) / 1000);
	return `${m}m ${sec}s`;
}

// percent is a ratio as "+12.3%" (signed) or "12.3%".
export function percent(r, signed = false) {
	const v = (r * 100).toFixed(Math.abs(r) < 0.1 ? 1 : 0);
	return (signed && r > 0 ? "+" : "") + v + "%";
}

// metric is a benchmark value in its unit, scaled: 1234567 ns is "1.23 ms".
export function metric(v, unit) {
	if (v === null || v === undefined) return "";
	const u = (unit || "").toLowerCase();
	if (u === "ns" || u === "ns/op") {
		if (v >= 1e9) return `${(v / 1e9).toPrecision(3)} s`;
		if (v >= 1e6) return `${(v / 1e6).toPrecision(3)} ms`;
		if (v >= 1e3) return `${(v / 1e3).toPrecision(3)} µs`;
		return `${v.toPrecision(3)} ns`;
	}
	if (u === "b" || u === "bytes" || u === "b/op") return bytes(Math.round(v));
	const n = Math.abs(v) >= 100 ? Math.round(v).toLocaleString("en-US") : Number(v.toPrecision(3)).toString();
	return unit ? `${n} ${unit}` : n;
}

// ext is a path's extension, lower case, without the dot.
export function ext(path) {
	const base = String(path || "").split("/").pop();
	const i = base.lastIndexOf(".");
	return i <= 0 ? "" : base.slice(i + 1).toLowerCase();
}

// basename and dirname split a slash path.
export function basename(path) {
	return String(path || "").split("/").pop();
}

export function dirname(path) {
	const parts = String(path || "").split("/");
	parts.pop();
	return parts.join("/");
}

// joinPath joins path parts, dropping empty ones.
export function joinPath(...parts) {
	return parts.filter((p) => p !== "" && p !== undefined && p !== null).join("/").replace(/\/+/g, "/");
}

// isImage reports a path the browser can show as an image.
export function isImage(path) {
	return ["png", "jpg", "jpeg", "gif", "webp", "avif", "ico", "bmp"].includes(ext(path));
}

// language names a path's language for highlighting.
export function language(path) {
	const e = ext(path);
	const base = basename(path).toLowerCase();
	if (e === "tin") return "tin";
	if (e === "md" || e === "markdown") return "markdown";
	if (e === "json") return "json";
	if (e === "sh" || e === "bash" || base === "makefile" || e === "mk") return "shell";
	if (e === "go") return "go";
	if (["js", "mjs", "cjs", "ts", "tsx", "jsx"].includes(e)) return "js";
	if (e === "css") return "css";
	if (e === "html" || e === "htm" || e === "svg" || e === "xml") return "html";
	if (e === "yml" || e === "yaml" || e === "toml") return "yaml";
	if (e === "sql") return "sql";
	if (e === "py") return "python";
	if (["c", "h", "cc", "cpp", "hpp", "rs", "java", "swift", "kt", "zig"].includes(e)) return "c";
	return "";
}

// visibilityLabel is how a visibility reads.
export function visibilityLabel(v) {
	return v === "private" ? "Private" : v === "internal" ? "Internal" : "Public";
}

// roleLabel is how a role reads.
export function roleLabel(role) {
	const map = { none: "No access", read: "Read", write: "Write", maintain: "Maintain", admin: "Admin", owner: "Owner", member: "Member" };
	return map[role] || (role ? role[0].toUpperCase() + role.slice(1) : "");
}

// stateLabel is how a review state reads.
export function stateLabel(state) {
	const map = { open: "Open", approved: "Approved", changes_requested: "Changes requested", landed: "Landed", abandoned: "Abandoned" };
	return map[state] || state || "";
}

// compareSpec reads a compare page's "base...head" (a lone head is compared with fallback, as is an empty side).
export function compareSpec(spec, fallback) {
	const s = spec || "";
	const at = s.indexOf("...");
	if (at < 0) return { from: fallback, to: s || fallback };
	return { from: s.slice(0, at) || fallback, to: s.slice(at + 3) || fallback };
}
