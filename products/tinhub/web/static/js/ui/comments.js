// Review comments: threads (a comment and its replies), with reply, resolve and Markdown bodies.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as fmt from "../lib/format.js";
import { markdown } from "./code.js";
import { composer } from "./composer.js";
import { avatar, btn, busy, time, badge } from "./kit.js";

// threads groups comments into [{root, replies}] in order of creation.
export function threads(comments) {
	const byId = new Map();
	const out = [];
	for (const c of [...comments].sort((a, b) => a.id - b.id)) {
		if (c.parent && byId.has(c.parent)) byId.get(c.parent).replies.push(c);
		else {
			const t = { root: c, replies: [] };
			byId.set(c.id, t);
			out.push(t);
		}
		if (c.parent && byId.has(c.parent)) byId.set(c.id, byId.get(c.parent));
	}
	return out;
}

function anchorText(c) {
	if (!c.file) return null;
	return [c.file, c.decl ? ` · ${c.decl}` : "", c.line ? ` · line ${c.line}` : ""].join("");
}

// thread renders one thread; act = {reply(root, body), resolve(root, bool), canWrite, link(c)}.
export function thread(t, act, { showAnchor = true } = {}) {
	const c = t.root;
	const replies = h("div.replies");
	const drawReplies = () => {
		replies.replaceChildren(
			...t.replies.map((r) => h("div.reply", { id: `c${r.id}` }, h("div.reply-head", {}, avatar(r.author_name || "?", "sm"), h("b", {}, r.author_name || "someone"), time(r.created_at)), h("div", {}, markdown(r.body)))),
		);
	};
	drawReplies();
	let box = null;
	const replyBtn = act.canWrite ? btn("Reply", { sm: true, ghost: true, icon: "comment" }) : null;
	const resolveBtn = act.canWrite ? btn(c.resolved ? "Unresolve" : "Resolve", { sm: true, ghost: true, icon: c.resolved ? "undo" : "check" }) : null;
	const foot = h("div.row", { style: { padding: "6px 10px", "border-top": "1px solid var(--border-faint)" } }, replyBtn, resolveBtn);
	if (replyBtn)
		replyBtn.onclick = () => {
			if (box) return box.focus();
			box = composer({
				placeholder: "Reply…",
				submitLabel: "Reply",
				compact: true,
				onCancel: () => {
					box.remove();
					box = null;
				},
				onSubmit: async (body) => {
					await act.reply(c, body);
					box.remove();
					box = null;
				},
			});
			foot.before(h("div", { style: { padding: "10px" } }, box));
			box.focus();
		};
	if (resolveBtn) resolveBtn.onclick = () => busy(resolveBtn, () => act.resolve(c, !c.resolved));
	const anchor = showAnchor && anchorText(c) ? h("div.anchor", {}, icon("fileCode", "sm"), act.link ? h("a", { href: act.link(c) }, anchorText(c)) : anchorText(c)) : null;
	return h(
		"div",
		{ class: ["comment", c.resolved && "resolved", c.outdated && "outdated"], id: `c${c.id}` },
		h(
			"div.comment-head",
			{},
			avatar(c.author_name || "?", "sm"),
			h("span.who", {}, c.author_name || "someone"),
			h("span.muted", {}, "commented"),
			time(c.created_at),
			h("span.spacer"),
			c.version ? badge("v" + c.version, "outline") : null,
			c.outdated ? badge("Outdated", "amber") : null,
			c.resolved ? badge("Resolved", "green") : null,
		),
		anchor,
		h("div.comment-body", {}, markdown(c.body)),
		t.replies.length ? replies : null,
		act.canWrite ? foot : null,
	);
}
