// Activity: events (pushes and review events, api eventsJson) as readable lines.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as fmt from "../lib/format.js";
import { avatar, time } from "./kit.js";

const branch = (name) => name.replace(/^refs\/(heads|tags)\//, "");

// describe is an event's icon, colour and sentence parts.
export function describe(e, { showRepo = true } = {}) {
	const p = e.payload || {};
	const repo = e.repo;
	const [o, n] = (repo || "/").split("/");
	const base = `/${o}/${n}`;
	const repoA = showRepo ? [" in ", h("a", { href: base }, h("b", {}, repo))] : [];
	const changeA = (ch) => h("a.mono", { href: `${base}/change/${ch}` }, fmt.shortChange(ch));
	switch (e.kind) {
		case "push": {
			const refs = p.refs || [];
			const parts = [];
			for (const r of refs.slice(0, 3)) {
				const isTag = r.name.startsWith("refs/tags/");
				const nm = branch(r.name);
				if (!r.new) parts.push(["deleted ", isTag ? "tag " : "branch ", h("b", {}, nm)]);
				else if (!r.old) parts.push(["created ", isTag ? "tag " : "branch ", h("a", { href: isTag ? `${base}/releases/${encodeURIComponent(nm)}` : `${base}/tree/${encodeURIComponent(nm)}` }, h("b", {}, nm))]);
				else parts.push(["pushed to ", h("a", { href: `${base}/commits/${encodeURIComponent(nm)}` }, h("b", {}, nm)), " ", h("a.hash", { href: `${base}/commit/${r.new}` }, fmt.short(r.new, 8))]);
			}
			const ch = (p.changes || []).length;
			if (ch) parts.push([fmt.plural(ch, "change version"), " for review"]);
			const sentence = parts.length ? parts.flatMap((x, i) => (i ? [", ", ...x] : x)) : ["pushed"];
			return { icon: "upload", color: "blue", body: [...sentence, ...repoA] };
		}
		case "review.opened":
			return { icon: "review", color: "green", body: ["opened review ", changeA(p.change), ...repoA] };
		case "review.voted":
			return { icon: p.detail === "approve" ? "thumbsUp" : "thumbsDown", color: p.detail === "approve" ? "green" : "amber", body: [p.detail === "approve" ? "approved " : "requested changes on ", changeA(p.change), p.version ? h("span.muted", {}, ` v${p.version}`) : null, ...repoA] };
		case "review.comment":
			return { icon: "comment", color: "", body: ["commented on ", h("a.mono", { href: `${base}/change/${p.change}#c${p.detail}` }, fmt.shortChange(p.change)), ...repoA] };
		case "review.check": {
			const [name, state] = String(p.detail || "").split(" ");
			const col = state === "success" ? "green" : state === "failure" || state === "error" ? "red" : "amber";
			return { icon: state === "success" ? "checkCircle" : state === "pending" ? "clock" : "xCircle", color: col, body: ["check ", h("b", {}, name || "?"), " is ", h("span", { class: col }, state || "?"), " on ", changeA(p.change), ...repoA], system: true };
		}
		case "review.landed":
			return { icon: "landed", color: "purple", body: ["landed ", changeA(p.change), p.detail ? [" as ", h("a.hash", { href: `${base}/commit/${p.detail}` }, fmt.short(p.detail, 8))] : null, ...repoA] };
		// a review.state event is only ever an abandon or a reopen (review.SetState)
		case "review.state":
			return { icon: p.state === "abandoned" ? "xCircle" : "refresh", color: p.state === "abandoned" ? "" : "green", body: [p.state === "abandoned" ? "abandoned " : "reopened ", changeA(p.change), ...repoA] };
		default:
			return { icon: "activity", color: "", body: [e.kind, ...repoA] };
	}
}

// item is one event as a feed row.
export function item(e, opts) {
	const d = describe(e, opts);
	const actor = e.actor || "";
	const who = actor.includes("@") ? actor.split("@")[0] : actor;
	return h(
		"div.feed-item",
		{},
		who && !d.system ? avatar(who, "md") : h("span.avatar.md", { class: "", style: { background: "var(--bg-sunken)", color: "var(--fg-muted)" } }, icon(d.icon, "sm")),
		h("div.grow", {}, h("div.what", {}, who && !d.system ? [h("a", { href: "/" + encodeURIComponent(who), class: "plain" }, h("b", {}, who)), " "] : null, d.body), h("div.when", {}, h("span", { class: d.color }, icon(d.icon, "sm")), " ", time(e.at))),
	);
}
