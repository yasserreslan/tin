// One commit: its message, author, signature, parents and change, and its diff against its first parent.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import { frame } from "./repo.js";
import * as filediff from "../ui/filediff.js";
import { avatar, copyButton, time, skeleton, errorBox, badge, callout } from "../ui/kit.js";

export async function render(ctx) {
	const { repo, body, base } = await frame(ctx, "code");
	const id = ctx.params.id;
	body.append(skeleton(6));
	const c = await api.get(api.R(repo.owner, repo.name)(`/commits/${encodeURIComponent(id)}`));
	if (!ctx.alive()) return;
	ctx.title(fmt.title(c.message) || fmt.short(c.id), `${repo.owner}/${repo.name}`);
	const rest = fmt.body(c.message);
	const parentLinks = (c.parents || []).map((p) => h("a.hash", { href: `${base}/commit/${p}` }, fmt.short(p, 8)));
	const head = h(
		"div.box.mb-4",
		{},
		h(
			"div",
			{ style: { padding: "20px 24px" } },
			h("h1", { style: { "font-size": "22px", "font-weight": "600" } }, fmt.title(c.message) || h("i.muted", {}, "No message")),
			rest ? h("pre", { style: { "white-space": "pre-wrap", margin: "12px 0 0", "font-family": "var(--font-sans)", color: "var(--fg-soft)" } }, rest) : null,
		),
		h(
			"div.box-head",
			{ style: { "border-top": "1px solid var(--border)", "border-bottom": "0", "flex-wrap": "wrap" } },
			avatar(c.author.name || c.author.email, "sm"),
			h("b", {}, c.author.name),
			h("span.muted", {}, "authored"),
			time(c.author.when),
			c.committer.name !== c.author.name || c.committer.when !== c.author.when ? [h("span.muted", {}, "· committed by"), h("b", {}, c.committer.name), time(c.committer.when)] : null,
			c.signed ? badge("Signed", "green outline", "signed") : badge("Unsigned", "outline"),
			h("span.spacer"),
			c.change ? h("a.chip", { href: `${base}/change/${c.change}` }, icon("change", "sm"), "change ", fmt.shortChange(c.change)) : null,
			parentLinks.length ? h("span.small.muted.row", {}, parentLinks.length > 1 ? "parents" : "parent", parentLinks) : h("span.small.muted", {}, "root commit"),
			h("span.small.muted.row", {}, "commit ", h("span.mono", {}, fmt.short(c.id, 12)), copyButton(c.id)),
			h("a.btn.sm", { href: `${base}/tree/${c.id}` }, icon("code", "sm"), "Browse files"),
		),
	);
	const conflicts = (c.conflicts || []).length ? callout("warn", h("b", {}, "This commit records conflicts"), " in ", c.conflicts.join(", "), ".") : null;
	const diffBox = h("div", {}, skeleton(6));
	body.replaceChildren(head, conflicts || "", diffBox);
	try {
		const cmp = await api.get(api.R(repo.owner, repo.name)("/compare"), { from: (c.parents || [])[0] || "", to: c.id });
		if (!ctx.alive()) return;
		diffBox.replaceChildren(filediff.view(repo, cmp.files || [], { links: (f) => (f.kind === "deleted" ? null : h("a.btn.sm.ghost", { href: `${base}/blob/${c.id}/${api.enc(f.path)}`, title: "View the file" }, icon("eye", "sm"))) }));
	} catch (err) {
		diffBox.replaceChildren(errorBox(err));
	}
}
