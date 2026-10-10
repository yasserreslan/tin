// A user's or an org's page: their repositories, and for an org its members, teams and settings (owners).

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import * as store from "../lib/store.js";
import { navigate } from "../lib/router.js";
import * as repos from "../ui/repos.js";
import { panel, input, select, userInput, saveButton } from "../ui/forms.js";
import { avatar, badge, btn, busy, callout, confirm, copyButton, empty, errorBox, field, more, segmented, skeleton, tabs, time, toast } from "../ui/kit.js";

export async function render(ctx) {
	const name = ctx.params.owner;
	ctx.main.append(h("div.container.page", {}, skeleton(6)));
	let o;
	try {
		o = await api.get(`/owners/${encodeURIComponent(name)}`);
	} catch (err) {
		if (err.status === 404) return ctx.notFound(`There is no user or org called ${name}.`);
		ctx.main.replaceChildren(h("div.container.page", {}, errorBox(err)));
		return;
	}
	if (!ctx.alive()) return;
	ctx.title(o.display ? `${o.display} (${o.name})` : o.name);
	const isOrg = o.kind === "org";
	const me = store.session.user;
	const self = me && me.name === o.name;
	const tab = ctx.query.tab || "repos";
	const q = (t) => `/${o.name}${t === "repos" ? "" : "?tab=" + t}`;
	const items = [
		{ key: "repos", label: "Repositories", icon: "repo", count: o.repos },
		isOrg ? { key: "members", label: "Members", icon: "users", count: o.members } : null,
		isOrg ? { key: "teams", label: "Teams", icon: "team", count: o.teams } : null,
		isOrg && o.admin ? { key: "settings", label: "Settings", icon: "settings" } : null,
	].filter(Boolean);
	const content = h("div");
	const side = h(
		"aside.profile-side",
		{},
		avatar(o.display || o.name, "xl", { square: isOrg }),
		h("h1", {}, o.display || o.name),
		h("div.handle", {}, o.name, " ", isOrg ? badge("Org", "outline", "org") : null),
		h(
			"div.col.small.muted",
			{ style: { gap: "6px", "margin-top": "14px" } },
			h("span.row", { style: { gap: "6px" } }, icon("clock", "sm"), "Joined ", fmt.date(o.created_at)),
			isOrg && o.role ? h("span.row", { style: { gap: "6px" } }, icon("shield", "sm"), `You're ${o.role === "owner" ? "an owner" : "a member"}`) : null,
		),
		self ? btn("Edit profile", { href: "/settings", icon: "edit", class: "mt-4" }) : null,
		self ? btn("New repository", { href: "/new", icon: "plus", primary: true, class: "mt-2" }) : null,
		isOrg && o.admin ? btn("New repository", { href: `/new?owner=${o.name}`, icon: "plus", primary: true, class: "mt-4" }) : null,
	);
	ctx.main.replaceChildren(
		h(
			"div.container.page",
			{},
			h("div.layout-profile", {}, side, h("div", {}, tabs(items.map((i) => ({ ...i, href: q(i.key), active: i.key === tab }))), h("div", { style: { "margin-top": "20px" } }, content))),
		),
	);
	const view = { repos: reposTab, members: membersTab, teams: teamsTab, settings: settingsTab }[tab];
	if (!view || !items.some((i) => i.key === tab)) return ctx.notFound();
	try {
		await view(ctx, o, content);
	} catch (err) {
		content.replaceChildren(errorBox(err));
	}
}

async function reposTab(ctx, o, content) {
	const filter = h("input.input", { type: "search", placeholder: "Find a repository…" });
	let style = store.pref("owner.view", "list");
	const list = h("div");
	const viewPick = segmented(
		[
			{ value: "list", icon: "list", title: "List" },
			{ value: "grid", icon: "grid", title: "Grid" },
		],
		style,
		(v) => {
			style = v;
			store.setPref("owner.view", v);
			draw();
		},
	);
	let all = [];
	const draw = () => {
		const t = filter.value.trim().toLowerCase();
		const shown = all.filter((r) => !t || r.name.includes(t) || (r.description || "").toLowerCase().includes(t));
		if (!all.length) return list.replaceChildren(h("div.box", {}, empty("repo", "No repositories yet", o.admin ? "Create one and push to it with tit." : `${o.name} has no repositories you can see.`, o.admin ? btn("New repository", { href: `/new${o.kind === "org" ? "?owner=" + o.name : ""}`, icon: "plus", primary: true }) : null)));
		if (!shown.length) return list.replaceChildren(h("div.box", {}, empty("search", "No repositories match", "Try a different filter.")));
		list.replaceChildren(style === "grid" ? h("div.repo-grid", {}, shown.map(repos.card)) : h("div.box", {}, shown.map((r) => repos.row(r, { showOwner: false }))));
	};
	content.replaceChildren(h("div.row.mb-4", {}, filter, viewPick), list);
	list.append(skeleton(5));
	let cursor = "";
	do {
		const r = await api.get("/repos", { owner: o.name, cursor, limit: 100 });
		all.push(...(r.repos || []));
		cursor = r.next || "";
	} while (cursor && all.length < 1000);
	if (!ctx.alive()) return;
	filter.oninput = draw;
	draw();
}

const roleBadge = (role) => (role === "owner" ? badge("Owner", "purple") : badge("Member", "outline"));

async function membersTab(ctx, o, content) {
	const draw = async () => {
		const r = await api.get(`/orgs/${o.name}/members`);
		const rows = (r.members || []).map((m) => {
			const kids = [avatar(m.display || m.name, "md"), h("div.grow", {}, h("a", { href: "/" + m.name }, h("b", {}, m.display || m.name)), h("div.small.muted", {}, m.name, m.since ? [" · joined ", time(m.since)] : null))];
			if (o.admin) {
				const s = select(
					[
						["member", "Member"],
						["owner", "Owner"],
					],
					m.role,
					{ style: { width: "120px", height: "30px" } },
				);
				s.onchange = () =>
					api.put(`/orgs/${o.name}/members/${encodeURIComponent(m.name)}`, { role: s.value }).then(
						() => toast(`${m.name} is now ${s.value === "owner" ? "an owner" : "a member"}`),
						(err) => toast(err.message, "error"),
					);
				const rm = btn("", { sm: true, ghost: true, icon: "x", title: `Remove ${m.name}` });
				rm.onclick = () =>
					busy(rm, async () => {
						if (!(await confirm({ title: `Remove ${m.name} from ${o.name}?`, message: "They leave its teams and lose the access those give.", confirmLabel: "Remove", danger: true }))) return;
						await api.del(`/orgs/${o.name}/members/${encodeURIComponent(m.name)}`);
						await draw();
					}).catch(() => {});
				kids.push(s, rm);
			} else kids.push(roleBadge(m.role));
			return h("div.box-row", {}, kids);
		});
		const who = userInput();
		const role = select(
			[
				["member", "Member"],
				["owner", "Owner"],
			],
			"member",
			{ style: { width: "120px" } },
		);
		const add = btn("Add member", { primary: true, icon: "plus" });
		add.onclick = () =>
			busy(add, async () => {
				const n = who.input.value.trim();
				if (!n) return;
				await api.put(`/orgs/${o.name}/members/${encodeURIComponent(n)}`, { role: role.value });
				toast(`${n} joined ${o.name}`);
				await draw();
			}).catch(() => {});
		content.replaceChildren(h("div.box", {}, rows.length ? rows : empty("users", "No members", "")), o.admin ? h("div.row.mt-4", {}, who, role, add) : null, o.admin ? h("p.small.muted.mt-2", {}, "Someone without an account yet? ", h("a", { href: "/admin/invites" }, "Invite them"), ".") : null);
	};
	await draw();
}

async function teamsTab(ctx, o, content) {
	const open = ctx.query.team || "";
	const draw = async () => {
		const r = await api.get(`/orgs/${o.name}/teams`);
		const teams = r.teams || [];
		const rows = await Promise.all(
			teams.map(async (t) => {
				const mem = h("div");
				const row = h(
					"div",
					{},
					h(
						"div.box-row",
						{},
						h("span.avatar.md.square", { style: { background: "var(--accent-soft)", color: "var(--accent)" } }, icon("team", "sm")),
						h("div.grow", {}, h("b", {}, t.name), h("div.small.muted", {}, fmt.plural(t.members, "member"), " · ", fmt.plural(t.repos, "repository", "repositories"))),
						btn(open === t.name ? "Hide members" : "Members", {
							sm: true,
							onclick: () => {
								ctx.query.team = open === t.name ? "" : t.name;
								navigate(`/${o.name}?tab=teams${ctx.query.team ? "&team=" + encodeURIComponent(ctx.query.team) : ""}`, { replace: true });
							},
						}),
						o.admin
							? (() => {
									const b = btn("", { sm: true, ghost: true, icon: "trash", title: `Delete ${t.name}` });
									b.onclick = () =>
										busy(b, async () => {
											if (!(await confirm({ title: `Delete team ${t.name}?`, message: "Its members lose the access the team gives.", confirmLabel: "Delete team", danger: true }))) return;
											await api.del(`/orgs/${o.name}/teams/${encodeURIComponent(t.name)}`);
											await draw();
										}).catch(() => {});
									return b;
								})()
							: null,
					),
					mem,
				);
				if (open === t.name) await drawMembers(o, t, mem, draw);
				return row;
			}),
		);
		const nm = input("", { placeholder: "team name", style: { "max-width": "240px" } });
		const add = btn("Create team", { primary: true, icon: "plus" });
		add.onclick = () =>
			busy(add, async () => {
				const n = nm.value.trim();
				if (!n) return;
				await api.post(`/orgs/${o.name}/teams`, { name: n });
				toast(`Team ${n} created`);
				await draw();
			}).catch(() => {});
		content.replaceChildren(h("div.box", {}, rows.length ? rows : empty("team", "No teams yet", "A team gives its members a role on the repositories it's added to.")), o.admin ? h("div.row.mt-4", {}, nm, add) : null);
	};
	await draw();
}

async function drawMembers(o, t, box, redraw) {
	const r = await api.get(`/orgs/${o.name}/teams/${encodeURIComponent(t.name)}/members`);
	const who = userInput({ placeholder: "add a member of " + o.name });
	const add = btn("Add", { sm: true, icon: "plus" });
	add.onclick = () =>
		busy(add, async () => {
			const n = who.input.value.trim();
			if (!n) return;
			await api.put(`/orgs/${o.name}/teams/${encodeURIComponent(t.name)}/members/${encodeURIComponent(n)}`);
			await redraw();
		}).catch(() => {});
	box.replaceChildren(
		h(
			"div",
			{ style: { padding: "4px 16px 14px 60px", background: "var(--bg-soft)", "border-top": "1px solid var(--border-faint)" } },
			(r.members || []).map((m) =>
				h(
					"div.row",
					{ style: { padding: "6px 0" } },
					avatar(m.name, "sm"),
					h("a.grow", { href: "/" + m.name }, m.display || m.name),
					o.admin
						? (() => {
								const b = btn("", { sm: true, ghost: true, icon: "x", title: "Remove from the team" });
								b.onclick = () =>
									busy(b, async () => {
										await api.del(`/orgs/${o.name}/teams/${encodeURIComponent(t.name)}/members/${encodeURIComponent(m.name)}`);
										await redraw();
									}).catch(() => {});
								return b;
							})()
						: null,
				),
			),
			(r.members || []).length ? null : h("p.small.muted", {}, "No members yet."),
			o.admin ? h("div.row", { style: { "margin-top": "8px" } }, who, add) : null,
		),
	);
}

async function settingsTab(ctx, o, content) {
	const display = input(o.display || "", { maxlength: 100 });
	const run = await api.get(`/orgs/${o.name}/runner`).catch(() => null);
	const signers = h("textarea.textarea.mono", { rows: 3, placeholder: "hex ed25519 public keys of the recording servers, one a line" });
	if (run) signers.value = (run.signers || []).join("\n");
	const optIn = btn(run && run.opted_in ? "Update" : "Opt in", { primary: true, icon: "runner" });
	optIn.onclick = () =>
		busy(optIn, async () => {
			await api.put(`/orgs/${o.name}/runner`, { signers: signers.value.split(/\s+/).filter(Boolean) });
			toast("Opted in");
			await settingsTab(ctx, o, content);
		}).catch(() => {});
	const optOut = run && run.opted_in ? btn("Opt out", { danger: true }) : null;
	if (optOut)
		optOut.onclick = () =>
			busy(optOut, async () => {
				if (!(await confirm({ title: "Opt out of behaviour runs?", message: "Reviews in this org stop replaying production requests.", confirmLabel: "Opt out", danger: true }))) return;
				await api.del(`/orgs/${o.name}/runner`);
				await settingsTab(ctx, o, content);
			}).catch(() => {});
	content.replaceChildren(
		panel(
			"Profile",
			null,
			field("Display name", display),
			saveButton(async () => {
				await api.patch(`/orgs/${o.name}`, { display: display.value.trim() });
				o.display = display.value.trim();
			}),
		),
		run
			? panel(
					"Behaviour runner",
					"Opting in lets tinhub's runner replay this org's recorded production requests against each review. Recording servers seal capsules for the runner's public key, so set it as their TIN_REPLAY_RECIPIENTS.",
					h(
						"div",
						{},
						run.opted_in
							? h(
									"div.mb-4",
									{},
									callout(run.current ? "ok" : "warn", run.current ? "Opted in with the current runner key." : "Opted in with an older key: update to seal for the current one."),
									h("div.field.mt-4", {}, h("label.label", {}, "Runner public key"), h("div.row", {}, h("code.grow", { style: { "word-break": "break-all" } }, run.public_key), copyButton(run.public_key)), h("div.hint", {}, "Key ", run.key_id)),
								)
							: null,
						field("Recording servers' signing keys", signers, "Optional. Capsules signed by other keys are not replayed."),
					),
					[optIn, h("span.spacer"), optOut],
				)
			: null,
	);
}
