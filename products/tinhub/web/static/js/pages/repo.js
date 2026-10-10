// The repository frame every repository page shares: the title, description, follow and clone buttons, and the tabs.
// frame(ctx, tab) loads the repository (cached briefly) and returns {repo, body} with body the page's container.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as store from "../lib/store.js";
import * as live from "../lib/live.js";
import * as fmt from "../lib/format.js";
import { addCommands, setRepo } from "../ui/layout.js";
import { avatar, btn, busy, copyButton, popover, tabs, toast, visibility } from "../ui/kit.js";

const cache = new Map();

export function forget(owner, name) {
	cache.delete(`${owner}/${name}`);
}

export async function load(owner, name) {
	const key = `${owner}/${name}`;
	const hit = cache.get(key);
	if (hit && Date.now() - hit.at < 15000) return hit.repo;
	const repo = await api.get(api.R(owner, name)());
	cache.set(key, { at: Date.now(), repo });
	return repo;
}

// base is a repository's page path.
export function base(repo) {
	return `/${repo.owner}/${repo.name}`;
}

export function cloneUrl(repo) {
	return `${store.site.url}/${repo.owner}/${repo.name}`;
}

function clonePanel(repo) {
	const url = cloneUrl(repo);
	const cmd = `tit clone ${url}`;
	return h(
		"div",
		{ style: { padding: "8px", width: "360px" } },
		h("div.strong.small", { style: { "margin-bottom": "8px" } }, "Clone with tit"),
		h("div.clone-box", {}, h("code", {}, url), copyButton(url, { sm: false })),
		h("div.copy-line", { style: { "margin-top": "10px" } }, h("code", {}, cmd), copyButton(cmd)),
		h("p.hint", { style: { "margin-top": "10px" } }, "Pushing needs your key to be registered here and write access."),
	);
}

export async function frame(ctx, tab, { wide = false } = {}) {
	const { owner, repo: name } = ctx.params;
	const repo = await load(owner, name);
	if (!ctx.alive()) throw new Error("navigated away");
	store.visit(`${repo.owner}/${repo.name}`);
	const b = base(repo);
	const signedIn = Boolean(store.session.user);
	const admin = repo.role === "admin";
	let following = repo.following;
	const followBtn = btn(following ? "Unfollow" : "Follow", { icon: following ? "bellOff" : "bell", sm: true, title: following ? "Stop following" : "Get notified about pushes and reviews" });
	followBtn.onclick = () =>
		busy(followBtn, async () => {
			if (following) await api.del(api.R(owner, name)("/subscription"));
			else await api.put(api.R(owner, name)("/subscription"));
			following = !following;
			repo.following = following;
			followBtn.replaceChildren(icon(following ? "bellOff" : "bell", "sm"), following ? "Unfollow" : "Follow");
			toast(following ? `Following ${owner}/${name}` : `Stopped following ${owner}/${name}`);
		});
	const cloneBtn = btn("Clone", { icon: "code", sm: true, primary: true, caret: true });
	cloneBtn.onclick = () => popover(cloneBtn, clonePanel(repo), { align: "right" });
	const items = [
		{ key: "code", label: "Code", icon: "code", href: b },
		{ key: "reviews", label: "Reviews", icon: "review", href: `${b}/reviews`, count: repo.open_reviews },
		{ key: "changes", label: "Changes", icon: "change", href: `${b}/changes`, count: repo.changes },
		{ key: "releases", label: "Releases", icon: "tag", href: `${b}/releases` },
		{ key: "bench", label: "Bench", icon: "gauge", href: `${b}/bench` },
		{ key: "replay", label: "Replay", icon: "replay", href: `${b}/replay` },
		{ key: "activity", label: "Activity", icon: "activity", href: `${b}/activity` },
		admin ? { key: "settings", label: "Settings", icon: "settings", href: `${b}/settings` } : null,
	]
		.filter(Boolean)
		.map((t) => ({ ...t, active: t.key === tab }));
	const head = h(
		"div.repo-head",
		{},
		h(
			"div.container",
			{},
			h(
				"div.repo-title",
				{},
				avatar(repo.owner, "md", { square: repo.owner_kind === "org" }),
				h("div.name", {}, h("a.owner", { href: "/" + repo.owner }, repo.owner), h("span.slash", {}, "/"), h("a.repo", { href: b }, repo.name)),
				visibility(repo.visibility),
				h("div.actions", {}, signedIn ? followBtn : null, cloneBtn),
			),
			repo.description ? h("p.repo-desc", {}, repo.description) : null,
			tabs(items),
		),
	);
	const body = h("div", { class: ["container", "page", wide && "wide"] });
	ctx.main.replaceChildren(head, body);
	ctx.title(`${repo.owner}/${repo.name}`);
	ctx.cleanup(setRepo({ owner: repo.owner, name: repo.name, rev: repo.default_branch || "main" }));
	ctx.cleanup(
		addCommands([
			{ label: `${repo.owner}/${repo.name}: Code`, icon: "code", href: b, group: "This repository" },
			{ label: `${repo.owner}/${repo.name}: Reviews`, icon: "review", href: `${b}/reviews`, group: "This repository" },
			{ label: `${repo.owner}/${repo.name}: Changes`, icon: "change", href: `${b}/changes`, group: "This repository" },
			{ label: `${repo.owner}/${repo.name}: History`, icon: "history", href: `${b}/commits`, group: "This repository" },
			{ label: `${repo.owner}/${repo.name}: Branches and tags`, icon: "branch", href: `${b}/refs`, group: "This repository" },
			{ label: `${repo.owner}/${repo.name}: Releases`, icon: "tag", href: `${b}/releases`, group: "This repository" },
			{ label: `${repo.owner}/${repo.name}: Bench`, icon: "gauge", href: `${b}/bench`, group: "This repository" },
			{ label: `${repo.owner}/${repo.name}: Replay`, icon: "replay", href: `${b}/replay`, group: "This repository" },
			admin ? { label: `${repo.owner}/${repo.name}: Settings`, icon: "settings", href: `${b}/settings`, group: "This repository" } : null,
		].filter(Boolean)),
	);
	return { repo, body, base: b };
}

// onLive calls fn with each live event of the repository while the page is shown.
export function onLive(ctx, repo, fn) {
	ctx.cleanup(live.follow(`repo:${repo.owner}/${repo.name}`, fn));
}

// revLabel is how a revision reads: a branch or tag name, or a short commit or change id.
export function revLabel(rev) {
	if (/^[0-9a-f]{64}$/.test(rev)) return fmt.short(rev);
	if (/^[a-z]{32}$/.test(rev)) return fmt.shortChange(rev);
	return rev.replace(/^refs\/(heads|tags)\//, "");
}
