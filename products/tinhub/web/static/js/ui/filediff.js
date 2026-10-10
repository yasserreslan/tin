// A list of changed files (from the compare and version-diff APIs) shown as line diffs computed in the browser from
// the two blobs: a summary, a unified or split toggle, and one collapsible box per file.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as df from "../lib/diff.js";
import * as fmt from "../lib/format.js";
import * as store from "../lib/store.js";
import { fileBlock, hunkTable } from "./code.js";
import { segmented, badge } from "./kit.js";

const blobCache = new Map();

export function blob(repo, id) {
	if (!id) return Promise.resolve("");
	const key = `${repo.owner}/${repo.name}/${id}`;
	if (!blobCache.has(key)) {
		const p = api.blobText(encodeURIComponent(repo.owner), encodeURIComponent(repo.name), id);
		blobCache.set(key, p);
		p.catch(() => blobCache.delete(key));
		if (blobCache.size > 400) blobCache.delete(blobCache.keys().next().value);
	}
	return blobCache.get(key);
}

const KIND_ICON = { added: "plus", deleted: "minus", modified: "edit", renamed: "rename", type_changed: "refresh" };
const KIND_COLOR = { added: "green", deleted: "red", modified: "amber", renamed: "blue", type_changed: "purple" };

// fileDiff computes one file's hunks: {hunks, add, del, binary}.
export async function fileDiff(repo, f) {
	const [a, b] = await Promise.all([blob(repo, f.old_blob), blob(repo, f.new_blob)]);
	if (a.includes("\u0000") || b.includes("\u0000")) return { hunks: [], add: 0, del: 0, binary: true };
	const rows = df.diffLines(df.splitLines(a), df.splitLines(b));
	const st = df.stats(rows);
	return { hunks: df.hunks(rows, 3), add: st.add, del: st.del, binary: false };
}

// view renders files; opts.title heads the summary, opts.links(f) gives a file's "view" link.
export function view(repo, files, { links, expandMax = 25, title = "" } = {}) {
	let split = store.pref("diff.split", false);
	const list = h("div");
	const totals = h("span.small.muted", {}, fmt.plural(files.length, "file") + " changed");
	const summary = h(
		"div.box.mb-4",
		{},
		h(
			"div.box-head",
			{},
			title ? h("b", {}, title) : null,
			totals,
			h("span.spacer"),
			segmented(
				[
					{ value: false, icon: "unified", label: "Unified" },
					{ value: true, icon: "split", label: "Split" },
				],
				split,
				(v) => {
					split = v;
					store.setPref("diff.split", v);
					draw();
				},
			),
		),
		h(
			"div",
			{ style: { "max-height": "260px", overflow: "auto" } },
			files.map((f) =>
				h(
					"a.box-row.plain",
					{ href: "#diff-" + encodeURIComponent(f.path), style: { "min-height": "34px", padding: "6px 16px" }, onclick: (e) => (e.preventDefault(), jump(f.path)) },
					h("span", { class: KIND_COLOR[f.kind] || "" }, icon(KIND_ICON[f.kind] || "file", "sm")),
					h("span.mono.small.ellipsis.grow", {}, f.old_path && f.old_path !== f.path ? `${f.old_path} → ${f.path}` : f.path),
				),
			),
		),
	);
	const blocks = new Map();
	let add = 0;
	let del = 0;
	const jump = (path) => {
		const el = blocks.get(path);
		if (!el) return;
		el.expand();
		el.scrollIntoView({ block: "start" });
		window.scrollBy(0, -70);
	};
	const draw = () => {
		list.replaceChildren();
		blocks.clear();
		add = 0;
		del = 0;
		files.forEach((f, i) => {
			const lang = fmt.language(f.path);
			const stat = h("span.row", { style: { gap: "6px" } });
			const el = fileBlock({
				path: f.path,
				oldPath: f.old_path,
				kind: f.kind,
				collapsed: i >= expandMax,
				badge: stat,
				actions: links ? [links(f)] : [],
				build: async () => {
					const d = await fileDiff(repo, f);
					stat.replaceChildren(h("span.stat-add", {}, "+" + d.add), h("span.stat-del", {}, "−" + d.del));
					add += d.add;
					del += d.del;
					totals.replaceChildren(fmt.plural(files.length, "file") + " changed, ", h("span.green", {}, `+${fmt.count(add)}`), " ", h("span.red", {}, `−${fmt.count(del)}`));
					if (d.binary) return h("div.box-empty", {}, "Binary file changed.");
					if (!d.hunks.length) return h("div.box-empty", {}, f.kind === "renamed" ? "Renamed without changes." : "No changes in content.");
					return h("div", { style: { overflow: "auto" } }, hunkTable(d.hunks, { lang, split }));
				},
			});
			blocks.set(f.path, el);
			list.append(el);
		});
		if (!files.length) list.append(h("div.box-empty.box", {}, "No files changed."));
	};
	draw();
	return h("div", {}, summary, list);
}

export function kindBadge(kind) {
	return badge(kind.replace("_", " "), KIND_COLOR[kind] || "");
}
