// Branches and tags.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import { frame } from "./repo.js";
import { refsOf } from "../ui/refs.js";
import { badge, copyButton, empty, skeleton, time } from "../ui/kit.js";

const enc = encodeURIComponent;

export async function render(ctx) {
	const { repo, body, base } = await frame(ctx, "code");
	ctx.title("Branches and tags", `${repo.owner}/${repo.name}`);
	body.append(skeleton(6));
	const refs = await refsOf(repo.owner, repo.name);
	if (!ctx.alive()) return;
	const branches = refs.filter((r) => r.kind === "branch").sort((a, b) => (a.short === repo.default_branch ? -1 : b.short === repo.default_branch ? 1 : a.short.localeCompare(b.short)));
	const tags = refs.filter((r) => r.kind === "tag");
	const row = (r) => {
		const meta = h("span.small.muted.grow.ellipsis");
		api.get(api.R(repo.owner, repo.name)("/log"), { rev: r.name, limit: 1 }).then(
			(x) => {
				const c = (x.commits || [])[0];
				if (c) meta.replaceChildren(h("a.plain", { href: `${base}/commit/${c.id}` }, c.title), " · ", c.author.name, " · ", time(c.committer.when));
			},
			() => {},
		);
		return h(
			"div.box-row",
			{},
			icon(r.kind === "tag" ? "tag" : "branch", "sm"),
			h("a.strong.mono", { href: r.kind === "tag" ? `${base}/releases/${enc(r.short)}` : `${base}/tree/${enc(r.short)}` }, r.short),
			r.short === repo.default_branch && r.kind === "branch" ? badge("default", "accent") : null,
			meta,
			h("a.hash", { href: `${base}/commit/${r.target}` }, fmt.short(r.target, 8)),
			copyButton(r.short, { title: "Copy the name" }),
			h("a.btn.sm", { href: `${base}/commits/${enc(r.short)}` }, icon("history", "sm"), "History"),
		);
	};
	body.replaceChildren(
		h("div.page-head", {}, h("h2", {}, "Branches and tags")),
		h("div.box.mb-4", {}, h("div.box-head", {}, icon("branch", "sm"), h("b", {}, "Branches"), h("span.counter", {}, String(branches.length))), branches.length ? branches.map(row) : h("div.box-empty", {}, "No branches.")),
		h("div.box", {}, h("div.box-head", {}, icon("tag", "sm"), h("b", {}, "Tags"), h("span.counter", {}, String(tags.length))), tags.length ? tags.map(row) : h("div.box-empty", {}, "No tags. Tag a release with ", h("code", {}, "tit tag"), ".")),
	);
	if (!refs.length) body.replaceChildren(empty("branch", "Nothing pushed yet", "Push a branch with tit to see it here."));
}
