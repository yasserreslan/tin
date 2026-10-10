// The code: a directory's files with its README, or one file, at a revision (a branch, tag, change or commit).

import { h, replace } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import * as hl from "../lib/highlight.js";
import { navigate } from "../lib/router.js";
import { frame, revLabel, cloneUrl } from "./repo.js";
import { refButton } from "../ui/refs.js";
import { fileView, markdown, parseLines } from "../ui/code.js";
import { btn, copyButton, empty, skeleton, time, avatar, segmented, errorBox, badge } from "../ui/kit.js";
import * as store from "../lib/store.js";
import { setRepo } from "../ui/layout.js";

const enc = encodeURIComponent;

function pathLink(base, kind, rev, path) {
	return `${base}/${kind}/${enc(rev)}${path ? "/" + api.enc(path) : ""}`;
}

function crumbs(repo, base, rev, path, isFile) {
	const parts = path ? path.split("/") : [];
	const out = [h("a", { href: pathLink(base, "tree", rev, "") }, repo.name)];
	parts.forEach((p, i) => {
		out.push(h("span.sep", {}, "/"));
		const sub = parts.slice(0, i + 1).join("/");
		if (i === parts.length - 1) out.push(h("span.last", {}, p));
		else out.push(h("a", { href: pathLink(base, "tree", rev, sub) }, p));
	});
	if (!isFile && parts.length) out.push(h("span.sep", {}, "/"));
	return h("nav.crumbs", { "aria-label": "Path" }, out);
}

function sortEntries(entries) {
	return [...entries].sort((a, b) => (a.mode === "dir") - (b.mode === "dir") === 0 ? a.name.localeCompare(b.name) : a.mode === "dir" ? -1 : 1);
}

// resolver of relative links in a README or Markdown file at dir.
function mdBase(base, rev, dir, apiBase) {
	const resolve = (p) => {
		const clean = p.split("#")[0].split("?")[0];
		const parts = (clean.startsWith("/") ? [] : dir ? dir.split("/") : []).concat(clean.split("/"));
		const out = [];
		for (const x of parts) {
			if (x === "" || x === ".") continue;
			if (x === "..") out.pop();
			else out.push(x);
		}
		return out.join("/");
	};
	return {
		link: (href) => {
			if (/^(https?:|mailto:)/i.test(href) || href.startsWith("#")) return href;
			const p = resolve(href);
			const hash = href.includes("#") ? "#" + href.split("#")[1] : "";
			return `${base}/blob/${enc(rev)}/${api.enc(p)}${hash}`;
		},
		image: (src) => {
			if (/^https?:/i.test(src)) return src;
			return `/api/v1${apiBase}/raw/${api.enc(resolve(src))}?rev=${enc(rev)}`;
		},
	};
}

// latestCommit is the newest commit that changed path. A page of a path's history reads a bounded part of the
// history, so one that changed long ago comes back empty with a cursor to go on from.
async function latestCommit(repo, rev, path) {
	let cursor = "";
	for (let i = 0; i < 20; i++) {
		const r = await api.get(api.R(repo.owner, repo.name)("/log"), { rev, path, cursor, limit: 1 });
		if ((r.commits || []).length || !r.next) return (r.commits || [])[0] || null;
		cursor = r.next;
	}
	return null;
}

function commitStrip(base, c, rev, path) {
	if (!c) return h("div.commit-strip", {}, h("span.muted", {}, "No commits."));
	return h(
		"div.commit-strip",
		{},
		avatar(c.author.name || c.author.email, "sm"),
		h("b", {}, c.author.name || c.author.email),
		h("a.msg.ellipsis.plain.grow", { href: `${base}/commit/${c.id}`, title: c.message }, c.title),
		c.signed ? h("span", { title: "Signed", class: "green" }, icon("signed", "sm")) : null,
		h("a.hash", { href: `${base}/commit/${c.id}` }, fmt.short(c.id, 8)),
		h("span.faint", {}, "·"),
		time(c.committer.when),
		h("a.btn.ghost.sm", { href: `${base}/commits/${enc(rev)}${path ? "/" + api.enc(path) : ""}` }, icon("history", "sm"), "History"),
	);
}

function emptyRepo(repo) {
	const url = cloneUrl(repo);
	const cmds = `tit clone ${url}\ncd ${repo.name}\necho "# ${repo.name}" > README.md\ntit add README.md\ntit commit -m "first commit"\ntit push`;
	return h(
		"div.card",
		{},
		h(
			"div.card-body",
			{},
			h("div.row-3", { style: { "margin-bottom": "16px" } }, h("div.empty", { style: { padding: "0" } }, h("div.art", {}, icon("repo")))),
			h("h2", {}, "This repository is empty"),
			h("p.muted", { style: { margin: "6px 0 20px" } }, "Push some commits with tit to get started."),
			h("div.clone-box", { style: { "margin-bottom": "16px" } }, h("code", {}, url), copyButton(url, { sm: false })),
			h("div.terminal", { style: { "margin-top": "0", "max-width": "none" } }, h("div.bar", {}, h("i"), h("i"), h("i")), h("pre", {}, cmds.split("\n").map((l) => [h("span.p", {}, "$ "), l, "\n"]))),
		),
	);
}

export async function render(ctx) {
	const { repo, body, base } = await frame(ctx, "code");
	const apiBase = api.R(repo.owner, repo.name)();
	const rev = ctx.params.rev || repo.default_branch || "main";
	// the palette finds files at the revision shown
	if (ctx.params.rev) ctx.cleanup(setRepo({ owner: repo.owner, name: repo.name, rev }));
	const path = (ctx.params.path || "").replace(/^\/+|\/+$/g, "");
	const blob = Boolean(ctx.route.blob);
	body.append(skeleton(8));
	let res;
	try {
		res = await api.get(api.R(repo.owner, repo.name)("/resolve"), { rev, path });
	} catch (err) {
		if (err.status === 404 && !repo.pushed_at && !path) {
			body.replaceChildren(emptyRepo(repo));
			return;
		}
		body.replaceChildren(errorBox(err));
		return;
	}
	if (!ctx.alive()) return;
	// a tree URL naming a file, or a blob URL naming a directory: show what it is
	if (res.kind === "blob" && !blob) {
		navigate(pathLink(base, "blob", rev, path), { replace: true });
		return;
	}
	if (res.kind === "tree" && blob) {
		navigate(pathLink(base, "tree", rev, path), { replace: true });
		return;
	}
	ctx.title(path ? `${path} at ${revLabel(rev)}` : "", `${repo.owner}/${repo.name}`);
	const go = (r) => navigate(pathLink(base, res.kind === "blob" ? "blob" : "tree", r, path));
	const bar = h(
		"div.file-bar",
		{},
		refButton(repo, rev, go),
		path ? crumbs(repo, base, rev, path, res.kind === "blob") : null,
		h("span.spacer"),
		!path ? h("a.btn", { href: `${base}/commits/${enc(rev)}` }, icon("history", "sm"), "History") : null,
		!path ? h("a.btn", { href: `${base}/refs` }, icon("branch", "sm"), "Branches") : null,
	);
	if (res.kind === "tree") renderTree(ctx, body, bar, repo, base, apiBase, rev, path, res);
	else renderBlob(ctx, body, bar, repo, base, apiBase, rev, path, res);
}

function renderTree(ctx, body, bar, repo, base, apiBase, rev, path, res) {
	const strip = h("div.commit-strip", {}, h("span.skeleton", { style: { width: "40%", height: "12px" } }));
	const rows = sortEntries(res.entries).map((e) => {
		const dir = e.mode === "dir";
		const sub = path ? `${path}/${e.name}` : e.name;
		return h("div.tree-row", {}, icon(dir ? "folder" : e.mode === "link" ? "link" : "file", dir ? "dir" : "file"), h("a.ellipsis", { href: pathLink(base, dir ? "tree" : "blob", rev, sub) }, e.name), h("span.meta", {}, e.mode === "exec" ? "exec" : ""));
	});
	const upRow = path ? h("div.tree-row", {}, icon("folder", "dir"), h("a", { href: pathLink(base, "tree", rev, path.split("/").slice(0, -1).join("/")) }, ".."), h("span")) : null;
	const box = h("div.box", {}, strip, upRow, rows.length ? rows : h("div.box-empty", {}, "This directory is empty."));
	const readme = res.entries.find((e) => /^readme(\.md|\.markdown|\.txt)?$/i.test(e.name) && e.mode !== "dir");
	const side = !path ? aboutPanel(repo, base, res) : null;
	const mainCol = h("div", {}, bar, box);
	body.replaceChildren(side ? h("div.layout-aside", {}, mainCol, side) : mainCol);
	latestCommit(repo, res.commit, path).then(
		(c) => ctx.alive() && strip.replaceWith(commitStrip(base, c, rev, path)),
		() => strip.replaceChildren(h("span.muted", {}, "History unavailable.")),
	);
	if (readme) {
		const rbox = h("div.box.readme", {}, h("div.box-head", {}, icon("fileText", "sm"), h("b", {}, readme.name)), skeleton(6));
		mainCol.append(rbox);
		const rp = path ? `${path}/${readme.name}` : readme.name;
		api.get(api.R(repo.owner, repo.name)("/resolve"), { rev: res.commit, path: rp }).then(
			(r) => {
				if (!ctx.alive()) return;
				const content = /\.txt$/i.test(readme.name) || !/\./.test(readme.name) ? h("pre.block", { style: { margin: "20px", border: "0" } }, r.text) : markdown(r.text, mdBase(base, rev, path, apiBase));
				content.classList.add("md");
				rbox.replaceChildren(rbox.firstChild, h("div", { style: { padding: "24px 32px" } }, content));
			},
			(err) => rbox.replaceChildren(rbox.firstChild, h("div.box-empty", {}, err.message)),
		);
	}
}

function aboutPanel(repo, base, res) {
	const url = cloneUrl(repo);
	return h(
		"aside.col",
		{ style: { gap: "16px" } },
		h("div", {}, h("h3", { style: { "font-size": "15px", "margin-bottom": "8px" } }, "About"), h("p.soft", {}, repo.description || h("span.muted", {}, "No description.")), h("div.col.small.muted", { style: { "margin-top": "14px", gap: "8px" } }, h("span.row", {}, icon("branch", "sm"), "Default branch ", h("b", {}, repo.default_branch)), h("span.row", {}, icon("database", "sm"), fmt.bytes(repo.size_bytes), repo.quota_bytes ? h("span.faint", {}, ` of ${fmt.bytes(repo.quota_bytes)}`) : null), h("span.row", {}, icon("clock", "sm"), repo.pushed_at ? time(repo.pushed_at, { prefix: "Pushed " }) : "Never pushed"), h("span.row", {}, icon("sparkle", "sm"), time(repo.created_at, { prefix: "Created " })), res.change ? h("span.row", {}, icon("change", "sm"), "Head change ", h("a.mono", { href: `${base}/change/${res.change}` }, fmt.shortChange(res.change))) : null)),
		h("hr", { style: { margin: "0" } }),
		h("div", {}, h("h3", { style: { "font-size": "15px", "margin-bottom": "8px" } }, "Clone"), h("div.clone-box", {}, h("code", {}, url), copyButton(url, { sm: false }))),
		h("hr", { style: { margin: "0" } }),
		h("div.col", { style: { gap: "4px" } }, [
			[`${base}/reviews`, "review", "Reviews", repo.open_reviews],
			[`${base}/changes`, "change", "Changes", repo.changes],
			[`${base}/releases`, "tag", "Releases"],
			[`${base}/bench`, "gauge", "Benchmarks"],
			[`${base}/refs`, "branch", "Branches and tags"],
		].map(([href, ic, label, n]) => h("a.row.plain.small", { href, style: { padding: "4px 0", color: "var(--fg-soft)" } }, icon(ic, "sm"), h("span.grow", {}, label), n ? h("span.counter", {}, String(n)) : null))),
	);
}

function renderBlob(ctx, body, bar, repo, base, apiBase, rev, path, res) {
	const lang = fmt.language(path);
	const rawUrl = `/api/v1${apiBase}/raw/${api.enc(path)}?rev=${enc(res.commit)}`;
	const lines = res.text ? res.text.split("\n").length - (res.text.endsWith("\n") ? 1 : 0) : 0;
	let wrap = store.pref("code.wrap", false);
	const isMd = lang === "markdown";
	let mode = isMd ? "preview" : "code";
	const content = h("div");
	const outlineBox = h("div.outline-col");
	const strip = h("div.commit-strip", {}, h("span.skeleton", { style: { width: "40%", height: "12px" } }));
	const head = h(
		"div.box-head",
		{},
		h("span.mono.small.nowrap", {}, res.binary ? "binary" : `${fmt.count(lines)} lines`),
		h("span.faint", {}, "·"),
		h("span.small.muted.nowrap", {}, fmt.bytes(res.size)),
		lang ? badge(lang, "outline") : null,
		h("span.spacer"),
		isMd ? segmented([{ value: "preview", label: "Preview" }, { value: "code", label: "Code" }], mode, (v) => ((mode = v), draw())) : null,
		!res.binary && !isMd ? btn("", { icon: "wrap", sm: true, ghost: true, title: "Wrap lines", onclick: () => ((wrap = !wrap), store.setPref("code.wrap", wrap), draw()) }) : null,
		res.text ? copyButton(() => res.text, { title: "Copy the file" }) : null,
		h("a.btn.sm", { href: rawUrl, "data-native": "", target: "_blank", rel: "noopener" }, "Raw"),
		h("a.btn.sm.icon", { href: rawUrl + "&download=1", "data-native": "", title: "Download", "aria-label": "Download" }, icon("download", "sm")),
	);
	const draw = () => {
		if (res.binary) {
			if (fmt.isImage(path) || fmt.ext(path) === "svg") replace(content, h("div.blob-image", {}, h("img", { src: rawUrl, alt: path })));
			else replace(content, empty("file", "Binary file", `${fmt.bytes(res.size)} of bytes that aren't text.`, h("a.btn", { href: rawUrl + "&download=1", "data-native": "" }, icon("download", "sm"), "Download")));
			return;
		}
		if (res.truncated) {
			replace(content, empty("fileText", "This file is too large to show", `${fmt.bytes(res.size)}. Open it raw to read it.`, h("a.btn", { href: rawUrl, "data-native": "", target: "_blank" }, "View raw")));
			return;
		}
		if (fmt.ext(path) === "svg" && mode !== "code") {
			replace(content, h("div.blob-image", {}, h("img", { src: rawUrl, alt: path })));
			return;
		}
		if (isMd && mode === "preview") {
			const dir = path.split("/").slice(0, -1).join("/");
			replace(content, h("div", { style: { padding: "24px 32px" } }, markdown(res.text, mdBase(base, rev, dir, apiBase))));
			return;
		}
		const sel = parseLines(ctx.hash);
		const view = fileView(res.text, {
			lang,
			wrap,
			selected: sel,
			onSelect: (s) => history.replaceState(null, "", `#L${s[0]}${s[1] !== s[0] ? "-L" + s[1] : ""}`),
		});
		replace(content, view);
		if (sel) setTimeout(() => view.scrollToLine(sel[0]), 30);
	};
	draw();
	if (lang === "tin" && res.text) {
		const decls = hl.outline(res.text);
		if (decls.length) {
			const filter = h("input.input", { placeholder: "Filter symbols", style: { height: "30px", "font-size": "12.5px", "margin-bottom": "8px" } });
			const list = h("div");
			const drawList = () => {
				const q = filter.value.toLowerCase();
				list.replaceChildren(
					...decls
						.filter((d) => !q || d.name.toLowerCase().includes(q) || d.recv.toLowerCase().includes(q))
						.map((d) =>
							h(
								"a",
								{ href: `#L${d.line}`, onclick: (e) => (e.preventDefault(), history.replaceState(null, "", `#L${d.line}`), (ctx.hash = `L${d.line}`), draw()) },
								h("span", { class: ["kind", d.kind] }, d.kind === "method" ? "fn" : d.kind),
								h("span.ellipsis", {}, d.recv ? h("span.faint", {}, d.recv + ".") : null, d.name),
							),
						),
				);
			};
			filter.oninput = drawList;
			drawList();
			outlineBox.append(h("div.outline.box", { style: { padding: "10px" } }, h("div.eyebrow", { style: { margin: "2px 4px 8px" } }, "Outline"), filter, list));
		}
	}
	const box = h("div.box", {}, strip, head, content);
	const mainCol = h("div", {}, bar, box);
	body.replaceChildren(outlineBox.childNodes.length ? h("div.layout-aside.narrow", {}, mainCol, outlineBox) : mainCol);
	latestCommit(repo, res.commit, path).then(
		(c) => ctx.alive() && strip.replaceWith(commitStrip(base, c, rev, path)),
		() => strip.replaceChildren(h("span.muted", {}, "History unavailable.")),
	);
	ctx.cleanup(() => {});
	const mod = { onHash: (hash) => ((ctx.hash = hash), !isMd || mode === "code" ? draw() : null) };
	onHashHook = mod.onHash;
}

let onHashHook = null;

export function onHash(hash) {
	if (onHashHook) onHashHook(hash);
}
