// Compare: what one branch, tag or commit did since it left another (/compare/base...head): its commits the base
// lacks and the files changed from where they split.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import { navigate } from "../lib/router.js";
import { frame, revLabel } from "./repo.js";
import { commitRow } from "./commits.js";
import { refButton } from "../ui/refs.js";
import * as filediff from "../ui/filediff.js";
import { btn, callout, empty, errorBox, skeleton } from "../ui/kit.js";

const enc = encodeURIComponent;

export async function render(ctx) {
	const { repo, body, base } = await frame(ctx, "code");
	const { from, to } = fmt.compareSpec(ctx.params.spec, repo.default_branch || "main");
	ctx.title(`Compare ${revLabel(from)}...${revLabel(to)}`, `${repo.owner}/${repo.name}`);
	const go = (a, b) => navigate(`${base}/compare/${enc(a)}...${enc(b)}`);
	const swap = btn("", { icon: "refresh", title: "Swap the two", onclick: () => go(to, from) });
	body.append(
		h("div.page-head", {}, h("div", {}, h("h2", {}, "Compare"), h("p.sub", {}, "What one branch, tag or commit did since it left another: its commits and the files they changed."))),
		h(
			"div.box.mb-4",
			{},
			h(
				"div.row",
				{ style: { "flex-wrap": "wrap", gap: "8px", padding: "10px 16px" } },
				h("span.small.muted", {}, "base"),
				refButton(repo, from, (r) => go(r, to)),
				icon("arrowLeft", "sm"),
				h("span.small.muted", {}, "compare"),
				refButton(repo, to, (r) => go(from, r)),
				swap,
			),
		),
	);
	const out = h("div", {}, skeleton(6));
	body.append(out);
	if (from === to) {
		out.replaceChildren(empty("branch", "Choose two different revisions", "Pick a branch, tag or commit to compare with the base."));
		return;
	}
	let cmp;
	try {
		cmp = await api.get(api.R(repo.owner, repo.name)("/compare"), { from, to, since: "base" });
	} catch (err) {
		if (ctx.alive()) out.replaceChildren(errorBox(err));
		return;
	}
	if (!ctx.alive()) return;
	const commits = cmp.commits || [];
	const files = cmp.files || [];
	if (!commits.length) {
		out.replaceChildren(empty("checkCircle", `${revLabel(from)} has everything in ${revLabel(to)}`, `${revLabel(to)} has no commits that ${revLabel(from)} lacks. Swap them to see what ${revLabel(from)} adds.`, btn("Swap", { icon: "refresh", onclick: () => go(to, from) })));
		return;
	}
	out.replaceChildren(
		h(
			"div.row.small.muted.mb-4",
			{ style: { gap: "14px", "flex-wrap": "wrap" } },
			h("span", {}, icon("commit", "sm"), " ", h("b", {}, cmp.more ? `${commits.length}+` : String(commits.length)), commits.length === 1 && !cmp.more ? " commit" : " commits"),
			h("span", {}, icon("file", "sm"), " ", h("b", {}, String(files.length)), files.length === 1 ? " file changed" : " files changed"),
			cmp.base ? h("span", {}, "since ", h("a.hash", { href: `${base}/commit/${cmp.base}` }, fmt.short(cmp.base, 8))) : null,
		),
		h("div.box.mb-4", {}, h("div.box-head", {}, icon("history", "sm"), h("b", {}, "Commits"), h("span.counter", {}, cmp.more ? `${commits.length}+` : String(commits.length))), commits.map((c) => commitRow(base, c)), cmp.more ? h("div.box-empty.small", {}, `The ${commits.length} newest; `, h("a", { href: `${base}/commits/${enc(to)}` }, "the history"), " has the rest.") : null),
		files.length ? filediff.view(repo, files, { title: "Files changed", links: (f) => (f.kind === "deleted" ? null : h("a.btn.sm.ghost", { href: `${base}/blob/${cmp.to}/${api.enc(f.path)}`, title: "View the file" }, icon("eye", "sm"))) }) : callout("info", "The commits change no files."),
	);
}
