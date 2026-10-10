// Releases: the repository's tags made by `tit ship`, each with its changelog, the changes it carries and whether
// its signature verifies; one release at /releases/<tag>.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import { frame } from "./repo.js";
import { markdown } from "../ui/code.js";
import { avatar, badge, btn, copyButton, empty, errorBox, more, skeleton, time } from "../ui/kit.js";

function signature(r) {
	if (r.verified) return badge("Verified", "green", "shieldCheck", { title: `Signed by ${r.signed_by}, with a key of their tinhub account` });
	if (r.signed) return badge("Unverified", "amber", "shield", { title: "Signed, but by no key of the tagger's tinhub account" });
	return null;
}

function releaseCard(repo, base, r, { full = false, latest = false } = {}) {
	const changes = r.changes || [];
	const shown = full ? changes : changes.slice(0, 8);
	// the notes are the message without its title and without the list of changes tit ship writes (shown below)
	const notes = fmt.body(r.message).split(/^Changes \(\d+\):\s*$/m)[0].trim();
	return h(
		"div.release",
		{},
		h(
			"div.release-side",
			{},
			h("a.chip", { href: `${base}/releases/${encodeURIComponent(r.name)}` }, icon("tag", "sm"), r.name),
			h("a.hash.small", { href: `${base}/commit/${r.commit}` }, icon("commit", "sm"), " ", fmt.short(r.commit, 8)),
			r.tagger && r.tagger.when ? h("div.small.muted", {}, time(r.tagger.when)) : null,
		),
		h(
			"div.card.grow",
			{},
			h(
				"div.card-body",
				{},
				h("div.row", { style: { "align-items": "center", gap: "10px", "flex-wrap": "wrap" } }, h("h2", { style: { margin: "0" } }, h("a.plain", { href: `${base}/releases/${encodeURIComponent(r.name)}` }, r.title || r.name)), latest ? badge("Latest", "green") : null, signature(r)),
				r.tagger && r.tagger.name ? h("div.row.small.muted", { style: { "margin-top": "8px", gap: "6px" } }, avatar(r.tagger.name, "sm"), h("b", {}, r.tagger.name), "shipped this", r.tagger.when ? time(r.tagger.when) : null) : null,
				notes ? h("div", { style: { "margin-top": "14px" } }, markdown(notes)) : null,
				changes.length
					? h(
							"div",
							{ style: { "margin-top": "16px" } },
							h("h4.small.muted", { style: { margin: "0 0 8px", "text-transform": "uppercase", "letter-spacing": ".04em" } }, fmt.plural(changes.length, "change")),
							h(
								"ul.release-changes",
								{},
								shown.map((c) => h("li", {}, h("a.mono.small", { href: `${base}/change/${c.change}` }, fmt.shortChange(c.change)), h("span", {}, c.title))),
							),
							!full && changes.length > shown.length ? h("a.small", { href: `${base}/releases/${encodeURIComponent(r.name)}` }, `and ${changes.length - shown.length} more`) : null,
						)
					: null,
				h(
					"div.row",
					{ style: { "margin-top": "16px", gap: "8px", "flex-wrap": "wrap" } },
					btn("Browse code", { href: `${base}/tree/${encodeURIComponent(r.name)}`, icon: "code", sm: true }),
					btn("History", { href: `${base}/commits/${encodeURIComponent(r.name)}`, icon: "history", sm: true }),
					h("span.spacer"),
					h("span.small.muted", {}, h("code", {}, `tit clone … && tit switch ${r.name}`)),
				),
			),
		),
	);
}

export async function render(ctx) {
	const { repo, body, base } = await frame(ctx, "releases");
	const R = api.R(repo.owner, repo.name);
	const tag = ctx.params.tag;
	if (tag) {
		ctx.title(tag, `${repo.owner}/${repo.name}`);
		body.append(skeleton(6));
		try {
			const r = await api.get(R(`/releases/${api.enc(tag)}`));
			if (!ctx.alive()) return;
			body.replaceChildren(h("div.crumbs.mb-4", {}, h("a", { href: `${base}/releases` }, "Releases"), h("span.sep", {}, "/"), h("span", {}, r.name)), releaseCard(repo, base, r, { full: true }));
		} catch (err) {
			if (err.status === 404) return ctx.notFound(`There is no release ${tag}.`);
			body.replaceChildren(errorBox(err));
		}
		return;
	}
	ctx.title("Releases", `${repo.owner}/${repo.name}`);
	const list = h("div.col", { style: { gap: "24px" } }, skeleton(6));
	body.append(h("div.page-head", {}, h("div", {}, h("h2", {}, "Releases"), h("p.sub", {}, "Tags made by ", h("code", {}, "tit ship"), ", with the changes each one carries.")), h("div.row", {}, copyButton("tit ship v1.0.0", { label: "tit ship", title: "Copy the command that cuts a release" }))), list);
	let cursor = "";
	let first = true;
	const load = async () => {
		const r = await api.get(R("/releases"), { cursor, limit: 20 });
		if (!ctx.alive()) return false;
		if (!cursor) list.replaceChildren();
		for (const x of r.releases || []) {
			list.append(releaseCard(repo, base, x, { latest: first }));
			first = false;
		}
		cursor = r.next || "";
		if (!list.childNodes.length) list.append(h("div.box", {}, empty("tag", "No releases yet", ["Cut one with ", h("code", {}, "tit ship v0.1.0"), ": it tags the trunk and writes the changelog from the changes since the last release."])));
		return Boolean(cursor);
	};
	try {
		if (await load()) list.after(more(load));
	} catch (err) {
		list.replaceChildren(errorBox(err));
	}
}
