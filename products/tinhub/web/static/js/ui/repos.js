// Repository cards and rows.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as fmt from "../lib/format.js";
import { visibility, time, avatar } from "./kit.js";

export function card(r) {
	return h(
		"a.repo-card.plain",
		{ href: `/${r.owner}/${r.name}` },
		h("div.row", {}, avatar(r.owner, "sm", { square: true }), h("span.name.ellipsis.grow", {}, h("span.muted", { style: { "font-weight": "500" } }, r.owner + " / "), r.name), visibility(r.visibility)),
		h("div.desc", {}, r.description || h("span.faint", {}, "No description")),
		h("div.foot", {}, icon("branch", "sm"), r.default_branch || "main", r.size_bytes ? [h("span", {}, "·"), fmt.bytes(r.size_bytes)] : null, h("span.spacer"), r.pushed_at ? time(r.pushed_at, { prefix: "pushed " }) : h("span", {}, "empty")),
	);
}

export function row(r, { showOwner = true } = {}) {
	return h(
		"a.box-row.plain",
		{ href: `/${r.owner}/${r.name}`, style: { "align-items": "flex-start", padding: "14px 16px" } },
		h("span", { style: { "margin-top": "2px", color: "var(--fg-muted)" } }, icon(r.visibility === "private" ? "lock" : "repo")),
		h(
			"div.grow",
			{},
			h("div.row", {}, h("span.title", {}, showOwner ? h("span.muted", { style: { "font-weight": "500" } }, r.owner + " / ") : null, r.name), visibility(r.visibility)),
			r.description ? h("div.meta", { style: { "margin-top": "4px" } }, r.description) : null,
			h("div.row.tiny.faint", { style: { "margin-top": "8px", gap: "12px" } }, h("span.row", { style: { gap: "4px" } }, icon("branch", "sm"), r.default_branch || "main"), r.size_bytes ? h("span", {}, fmt.bytes(r.size_bytes)) : null, r.pushed_at ? time(r.pushed_at, { prefix: "Updated " }) : h("span", {}, "Empty")),
		),
	);
}
