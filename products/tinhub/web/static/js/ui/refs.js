// The branch and tag picker.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import { picker, btn } from "./kit.js";
import { revLabel } from "../pages/repo.js";

const lists = new Map();

// refsOf lists a repository's branches and tags: [{name, short, kind, target}].
export async function refsOf(owner, name) {
	const key = `${owner}/${name}`;
	const hit = lists.get(key);
	if (hit && Date.now() - hit.at < 10000) return hit.refs;
	const out = [];
	let cursor = "";
	for (let i = 0; i < 10; i++) {
		const r = await api.get(api.R(owner, name)("/refs"), { cursor, limit: 200 });
		for (const x of r.refs || []) {
			const kind = x.name.startsWith("refs/heads/") ? "branch" : x.name.startsWith("refs/tags/") ? "tag" : "other";
			if (kind === "other") continue;
			out.push({ name: x.name, short: x.name.replace(/^refs\/(heads|tags)\//, ""), kind, target: x.target });
		}
		cursor = r.next || "";
		if (!cursor) break;
	}
	lists.set(key, { at: Date.now(), refs: out });
	return out;
}

// refButton is the picker's button; onPick(shortName) is called with the chosen branch or tag.
export function refButton(repo, rev, onPick) {
	const isBranch = !/^[0-9a-f]{64}$/.test(rev) && !/^[a-z]{32}$/.test(rev);
	const b = btn(revLabel(rev), { icon: isBranch ? "branch" : "commit", caret: true, sm: false, title: "Switch branch or tag" });
	b.style.maxWidth = "260px";
	b.onclick = async () => {
		let refs = [];
		try {
			refs = await refsOf(repo.owner, repo.name);
		} catch {}
		const branches = refs.filter((r) => r.kind === "branch");
		const tags = refs.filter((r) => r.kind === "tag");
		const items = [];
		if (branches.length) items.push({ head: "Branches" }, ...branches.map((r) => ({ value: r.short, label: r.short, icon: "branch", sub: r.short === repo.default_branch ? "default" : "" })));
		if (tags.length) items.push({ head: "Tags" }, ...tags.map((r) => ({ value: r.short, label: r.short, icon: "tag" })));
		picker(b, { items, placeholder: "Find a branch or tag", current: revLabel(rev), onPick, footer: h("a.menu-item", { href: `/${repo.owner}/${repo.name}/refs` }, icon("list"), "View all branches and tags") });
	};
	return b;
}
