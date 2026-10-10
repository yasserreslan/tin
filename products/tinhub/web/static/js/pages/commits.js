// History: the commits from a revision, newest first, grouped by day; with a path, those that changed it.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import { navigate } from "../lib/router.js";
import { frame, revLabel } from "./repo.js";
import { refButton } from "../ui/refs.js";
import { avatar, copyButton, time, more, skeleton, empty, errorBox } from "../ui/kit.js";

const enc = encodeURIComponent;

export function commitRow(base, c) {
	return h(
		"div.box-row",
		{ style: { "align-items": "flex-start", padding: "12px 16px" } },
		avatar(c.author.name || c.author.email, "md"),
		h(
			"div.grow",
			{ style: { "min-width": "0" } },
			h("a.plain.strong.ellipsis", { href: `${base}/commit/${c.id}`, style: { display: "block" }, title: c.message }, c.title || h("i.muted", {}, "No message")),
			h(
				"div.row.small.muted",
				{ style: { "margin-top": "4px", "flex-wrap": "wrap" } },
				h("b", { style: { color: "var(--fg-soft)" } }, c.author.name || c.author.email),
				c.committer && c.committer.name !== c.author.name ? h("span", {}, "with ", c.committer.name) : null,
				time(c.committer.when),
				c.change ? h("a.chip", { href: `${base}/change/${c.change}`, title: "Change " + c.change }, icon("change", "sm"), fmt.shortChange(c.change)) : null,
				// with the commit's details, where it wraps on a phone rather than squeezing the title
				c.signed ? h("span.badge.green.outline", { title: "Signed by its author's key" }, icon("signed"), "Signed") : null,
			),
		),
		h("a.btn.sm.mono", { href: `${base}/commit/${c.id}` }, fmt.short(c.id, 8)),
		copyButton(c.id, { title: "Copy the commit id" }),
		h("a.btn.sm.icon.ghost", { href: `${base}/tree/${c.id}`, title: "Browse the files at this commit", "aria-label": "Browse files" }, icon("code", "sm")),
	);
}

export async function render(ctx) {
	const { repo, body, base } = await frame(ctx, "code");
	const rev = ctx.params.rev || repo.default_branch || "main";
	const path = (ctx.params.path || "").replace(/^\/+|\/+$/g, "");
	ctx.title(`History of ${path || revLabel(rev)}`, `${repo.owner}/${repo.name}`);
	const list = h("div");
	body.append(
		h(
			"div.page-head",
			{ style: { "align-items": "center" } },
			h("h2", {}, icon("history", "lg"), " History"),
			path ? h("span.chip", {}, path) : null,
			h("div.actions", {}, refButton(repo, rev, (r) => navigate(`${base}/commits/${enc(r)}${path ? "/" + api.enc(path) : ""}`)), path ? h("a.btn", { href: `${base}/commits/${enc(rev)}` }, icon("x", "sm"), "All files") : null),
		),
		list,
	);
	list.append(skeleton(8));
	let cursor = "";
	let lastDay = "";
	let box = null;
	const load = async () => {
		let r = await api.get(api.R(repo.owner, repo.name)("/log"), { rev, path, cursor, limit: 40 });
		// a page of a path's history reads a bounded part of the history: go on while it found nothing
		for (let i = 0; !(r.commits || []).length && r.next && i < 20; i++) {
			r = await api.get(api.R(repo.owner, repo.name)("/log"), { rev, path, cursor: r.next, limit: 40 });
		}
		if (!cursor) list.replaceChildren();
		for (const c of r.commits || []) {
			const day = fmt.date(c.committer.when);
			if (day !== lastDay) {
				lastDay = day;
				list.append(h("div.row.small.muted", { style: { margin: "20px 0 8px", gap: "8px" } }, icon("commit", "sm"), "Commits on ", day));
				box = h("div.box");
				list.append(box);
			}
			box.append(commitRow(base, c));
		}
		if (!list.childNodes.length) list.append(empty("history", "No commits", path ? `Nothing at ${revLabel(rev)} changed ${path}.` : "This revision has no commits."));
		cursor = r.next || "";
		return Boolean(cursor);
	};
	try {
		if (await load()) body.append(more(load));
	} catch (err) {
		// a repository nothing was pushed to has no history yet, not a missing branch
		if (err.status === 404 && !repo.pushed_at) list.replaceChildren(empty("history", "No commits yet", "Push a branch with tit and its history shows up here.", h("a.btn", { href: base }, icon("code", "sm"), "How to push")));
		else list.replaceChildren(errorBox(err));
	}
}
