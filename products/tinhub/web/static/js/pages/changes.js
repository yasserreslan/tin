// Changes: every change pushed to the repository with its newest version, newest first.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import { frame } from "./repo.js";
import { title } from "../ui/commits.js";
import { avatar, empty, skeleton, time, more, errorBox } from "../ui/kit.js";

export function changeRow(repo, base, c) {
	const v = c.newest;
	return h(
		"a.box-row.plain",
		{ href: `${base}/change/${c.change}`, style: { "align-items": "flex-start", padding: "12px 16px" } },
		h("span.muted", { style: { "margin-top": "2px" } }, icon("change")),
		h(
			"div.grow",
			{ style: { "min-width": "0" } },
			h("div.title.ellipsis", {}, title(repo, v.commit, fmt.shortChange(c.change))),
			h("div.row.small.muted", { style: { "margin-top": "4px", gap: "6px", "flex-wrap": "wrap" } }, h("span.mono", {}, fmt.shortChange(c.change)), "·", v.pushed_by ? [h("b", {}, v.pushed_by), " pushed"] : "pushed", time(v.time)),
		),
		h("span.badge.outline", {}, "v" + v.version),
		h("span.hash", {}, fmt.short(v.commit, 8)),
		v.pushed_by ? avatar(v.pushed_by, "sm") : null,
	);
}

export async function render(ctx) {
	const { repo, body, base } = await frame(ctx, "changes");
	ctx.title("Changes", `${repo.owner}/${repo.name}`);
	const list = h("div.box", {}, skeleton(6));
	body.append(h("div.page-head", { style: { "align-items": "center" } }, h("div", {}, h("h2", {}, "Changes"), h("p.sub", {}, "A change keeps its id through every amend and rebase; each push of it is a new version.")), h("div.actions", {}, store_user(ctx) ? h("a.btn", { href: `${base}/stacks/${ctx.user.name}` }, icon("stack", "sm"), "Your stacks") : null)), list);
	let cursor = "";
	const load = async () => {
		const r = await api.get(api.R(repo.owner, repo.name)("/changes"), { cursor, limit: 50 });
		if (!cursor) list.replaceChildren();
		for (const c of r.changes || []) list.append(changeRow(repo, base, c));
		if (!list.childNodes.length) list.append(empty("change", "No changes yet", "Changes appear when someone pushes a branch whose commits carry change ids (every tit commit does)."));
		cursor = r.next || "";
		return Boolean(cursor);
	};
	try {
		if (await load()) body.append(more(load));
	} catch (err) {
		list.replaceChildren(errorBox(err));
	}
}

function store_user(ctx) {
	return ctx.user;
}
