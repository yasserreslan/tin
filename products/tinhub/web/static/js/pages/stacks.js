// A person's stacks: their changes drawn as chains, each change on the one below it, bottom first.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import { navigate } from "../lib/router.js";
import { frame } from "./repo.js";
import { title } from "../ui/commits.js";
import { avatar, empty, skeleton, time, errorBox, stateBadge } from "../ui/kit.js";

// chains groups changes into stacks by their commits' parents: each chain bottom first.
export function chains(changes) {
	const byCommit = new Map(changes.map((c) => [c.newest.commit, c]));
	const above = new Map();
	for (const c of changes) {
		const p = (c.parents || [])[0];
		if (p && byCommit.has(p)) above.set(p, c);
	}
	const bottoms = changes.filter((c) => !byCommit.has((c.parents || [])[0]));
	return bottoms.map((b) => {
		const chain = [b];
		let cur = b;
		const seen = new Set([b.change]);
		while (above.has(cur.newest.commit)) {
			cur = above.get(cur.newest.commit);
			if (seen.has(cur.change)) break;
			seen.add(cur.change);
			chain.push(cur);
		}
		return chain;
	});
}

export async function render(ctx) {
	const { repo, body, base } = await frame(ctx, "changes");
	const user = ctx.params.user || (ctx.user && ctx.user.name);
	if (!user) {
		navigate(`${base}/changes`, { replace: true });
		return;
	}
	ctx.title(`${user}'s stacks`, `${repo.owner}/${repo.name}`);
	const out = h("div", {}, skeleton(6));
	body.append(h("div.page-head", {}, avatar(user, "lg"), h("div", {}, h("h2", {}, user, "'s stacks"), h("p.sub", {}, "The changes ", user, " pushed last, as stacks: each change sits on the one below it."))), out);
	try {
		const r = await api.get(api.R(repo.owner, repo.name)(`/stacks/${encodeURIComponent(user)}`), { limit: 200 });
		if (!ctx.alive()) return;
		const cs = chains(r.changes || []);
		const reviews = await api.get(api.R(repo.owner, repo.name)("/reviews"), { state: "" }).catch(() => ({ reviews: [] }));
		const states = new Map((reviews.reviews || []).map((x) => [x.change, x.state]));
		out.replaceChildren();
		if (!cs.length) {
			out.append(empty("stack", "No stacks", `${user} has no changes here.`));
			return;
		}
		cs.sort((a, b) => b[b.length - 1].newest.time - a[a.length - 1].newest.time);
		for (const chain of cs) {
			const top = chain[chain.length - 1];
			out.append(
				h(
					"div.box.mb-4",
					{},
					h("div.box-head", {}, icon("stack", "sm"), h("b", {}, fmt.plural(chain.length, "change")), h("span.muted", {}, "on ", h("a.hash", { href: `${base}/commit/${(chain[0].parents || [])[0] || ""}` }, fmt.short((chain[0].parents || [])[0] || "", 8))), h("span.spacer"), time(top.newest.time, { prefix: "updated " })),
					h(
						"div.stack-graph",
						{ style: { padding: "6px 0" } },
						[...chain].reverse().map((c) =>
							h(
								"a.stack-item.plain",
								{ href: `${base}/change/${c.change}`, class: states.get(c.change) === "landed" ? "landed" : "" },
								h("span.rail"),
								h("div.grow", { style: { "font-family": "var(--font-sans)", "min-width": "0" } }, h("div.strong.ellipsis", {}, title(repo, c.newest.commit, fmt.shortChange(c.change))), h("div.small.muted.mono", {}, fmt.shortChange(c.change), " · v", String(c.newest.version))),
								states.get(c.change) ? stateBadge(states.get(c.change)) : h("span.badge.outline", {}, "no review"),
							),
						),
						h("div.stack-item.base", {}, h("span.rail"), h("span.small.muted", { style: { "font-family": "var(--font-sans)" } }, "base ", h("span.mono", {}, fmt.short((chain[0].parents || [])[0] || "", 10)))),
					),
				),
			);
		}
	} catch (err) {
		out.replaceChildren(errorBox(err));
	}
}
