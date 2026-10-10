// A repository's settings (admins): general (name, description, visibility, default branch, deletion), access
// (collaborators and teams), reviews (approvals and required checks), webhooks and their deliveries, the behaviour
// runner, replay (retention and grants), the mirror and the audit log.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import { navigate } from "../lib/router.js";
import { frame, forget } from "./repo.js";
import { refsOf } from "../ui/refs.js";
import { sidenav, panel, input, textarea, select, checkbox, userInput, saveButton } from "../ui/forms.js";
import { avatar, badge, btn, busy, callout, confirm, copyButton, dialog, empty, errorBox, field, kv, more, refused, signInFirst, skeleton, time, toast } from "../ui/kit.js";

const SECTIONS = [
	{ key: "general", label: "General", icon: "settings" },
	{ key: "access", label: "Access", icon: "users" },
	{ key: "reviews", label: "Reviews", icon: "review" },
	{ key: "webhooks", label: "Webhooks", icon: "webhook" },
	{ group: "Behaviour" },
	{ key: "runner", label: "Runner", icon: "runner" },
	{ key: "replay", label: "Replay", icon: "replay" },
	{ group: "More" },
	{ key: "mirror", label: "Mirror", icon: "mirror" },
	{ key: "audit", label: "Audit log", icon: "scroll" },
];

const HOOK_KINDS = ["push", "review.opened", "review.voted", "review.comment", "review.check", "review.landed", "review.state"];

export async function render(ctx) {
	const { repo, body, base } = await frame(ctx, "settings");
	const section = ctx.params.section || "general";
	ctx.title("Settings", `${repo.owner}/${repo.name}`);
	if (repo.role !== "admin") {
		body.append(h("div.box", {}, refused(null, "Settings are for admins", `Ask an admin of ${repo.owner}/${repo.name} to change its settings.`, signInFirst("Sign in to change settings", `Settings of ${repo.owner}/${repo.name} are for its admins.`))));
		return;
	}
	const S = SECTIONS.find((s) => s.key === section);
	if (!S) return ctx.notFound();
	const main = h("div", {}, skeleton(6));
	body.append(h("div.layout-sidebar", {}, sidenav(SECTIONS.map((s) => (s.group ? s : { ...s, href: `${base}/settings${s.key === "general" ? "" : "/" + s.key}` })), section), main));
	const R = api.R(repo.owner, repo.name);
	const env = { ctx, repo, base, R, main };
	try {
		await PAGES[section](env);
	} catch (err) {
		main.replaceChildren(errorBox(err));
	}
}

const PAGES = {
	// ---- general ----
	async general({ repo, base, R, main }) {
		const refs = await refsOf(repo.owner, repo.name).catch(() => []);
		const branches = refs.filter((r) => r.kind === "branch").map((r) => r.short);
		if (!branches.includes(repo.default_branch)) branches.unshift(repo.default_branch);
		const name = input(repo.name, { spellcheck: "false", autocomplete: "off" });
		const desc = textarea(repo.description || "", { rows: 3, maxlength: 350, placeholder: "What this repository is for" });
		const branch = select(branches, repo.default_branch);
		const vis = select(
			[
				["public", "Public: anyone can read it"],
				["private", "Private: only people given access"],
			],
			repo.visibility,
		);
		const save = async () => {
			const patch = {};
			if (name.value.trim() !== repo.name) patch.name = name.value.trim();
			if (desc.value !== (repo.description || "")) patch.description = desc.value;
			if (branch.value !== repo.default_branch) patch.default_branch = branch.value;
			if (vis.value !== repo.visibility) {
				if (vis.value === "private" && !(await confirm({ title: "Change visibility?", message: `${repo.owner}/${repo.name} becomes ${vis.value}. People without access lose it, along with their follows.`, confirmLabel: "Change" }))) return;
				patch.visibility = vis.value;
			}
			if (!Object.keys(patch).length) return;
			const r = await api.patch(R(""), patch);
			forget(repo.owner, repo.name);
			if (patch.name) navigate(`/${repo.owner}/${r.name || patch.name}/settings`);
			else Object.assign(repo, patch);
		};
		const del = btn("Delete this repository", { danger: true, icon: "trash" });
		del.onclick = () =>
			busy(del, async () => {
				const full = `${repo.owner}/${repo.name}`;
				if (!(await confirm({ title: `Delete ${full}?`, message: "Its history, reviews, benchmarks and capsules go with it. This can't be undone.", confirmLabel: "Delete repository", danger: true, typed: full }))) return;
				await api.del(R(""));
				forget(repo.owner, repo.name);
				toast(`Deleted ${full}`);
				navigate(`/${repo.owner}`);
			}).catch(() => {});
		main.replaceChildren(
			panel(
				"General",
				null,
				h("div", {}, field("Name", name, `Renaming moves the repository to ${location.host}/${repo.owner}/<name>; clones need their remote updated.`), field("Description", desc), field("Default branch", branch, "Reviews target it, and the code page opens on it."), field("Visibility", vis)),
				saveButton(save),
			),
			panel("Usage", null, kv([["Size", `${fmt.bytes(repo.size_bytes)} of ${fmt.bytes(repo.quota_bytes)}`], ["Created", time(repo.created_at)], ["Last push", repo.pushed_at ? time(repo.pushed_at) : "never"]])),
			h("div.box.danger-zone", {}, h("div.box-head", {}, h("b.red", {}, "Danger zone")), h("div.box-row", {}, h("div", {}, h("b", {}, "Delete this repository"), h("div.small.muted", {}, "Everything in it is deleted for good.")), del)),
		);
	},

	// ---- access ----
	async access({ repo, R, main }) {
		const isOrg = repo.owner_kind === "org";
		const draw = async () => {
			const a = await api.get(R("/collaborators"));
			const teams = isOrg ? ((await api.get(`/orgs/${repo.owner}/teams`).catch(() => ({ teams: [] }))).teams || []) : [];
			const roleSel = (role, onChange) => {
				const s = select(
					[
						["read", "Read"],
						["write", "Write"],
						["admin", "Admin"],
					],
					role,
					{ style: { width: "110px", height: "30px" } },
				);
				s.onchange = () => onChange(s.value).catch((err) => toast(err.message, "error"));
				return s;
			};
			const removeBtn = (what, fn) => {
				const b = btn("", { sm: true, ghost: true, icon: "x", title: "Remove " + what });
				b.onclick = () =>
					busy(b, async () => {
						if (!(await confirm({ title: `Remove ${what}?`, message: "They lose the access this grant gives.", confirmLabel: "Remove", danger: true }))) return;
						await fn();
						await draw();
					}).catch(() => {});
				return b;
			};
			const who = userInput();
			const role = select(["read", "write", "admin"], "write", { style: { width: "110px" } });
			const add = btn("Add", { primary: true, icon: "plus" });
			add.onclick = () =>
				busy(add, async () => {
					const n = who.input.value.trim();
					if (!n) return;
					await api.put(R(`/collaborators/${encodeURIComponent(n)}`), { role: role.value });
					toast(`${n} can ${role.value === "read" ? "read" : role.value === "write" ? "push to" : "administer"} ${repo.name}`);
					await draw();
				}).catch(() => {});
			const rows = [
				...(a.owners || []).map((u) => h("div.box-row", {}, avatar(u.name, "md"), h("div.grow", {}, h("a", { href: "/" + u.name }, h("b", {}, u.display || u.name)), h("div.small.muted", {}, u.name)), badge(isOrg ? "Org owner" : "Owner", "purple"))),
				...(a.users || []).map((u) =>
					h(
						"div.box-row",
						{},
						avatar(u.name, "md"),
						h("div.grow", {}, h("a", { href: "/" + u.name }, h("b", {}, u.display || u.name)), h("div.small.muted", {}, u.name)),
						roleSel(u.role, async (v) => {
							await api.put(R(`/collaborators/${encodeURIComponent(u.name)}`), { role: v });
							toast(`${u.name} is now ${v}`);
						}),
						removeBtn(u.name, () => api.del(R(`/collaborators/${encodeURIComponent(u.name)}`))),
					),
				),
			];
			const teamBox = isOrg
				? (() => {
						const pick = select(
							teams.map((t) => t.name),
							"",
							{ style: { width: "180px" } },
						);
						const trole = select(["read", "write", "admin"], "read", { style: { width: "110px" } });
						const addT = btn("Add team", { icon: "plus" });
						addT.onclick = () =>
							busy(addT, async () => {
								if (!pick.value) return;
								await api.put(R(`/teams/${encodeURIComponent(pick.value)}`), { role: trole.value });
								await draw();
							}).catch(() => {});
						return panel(
							"Teams",
							`Teams of ${repo.owner} with access; every member gets the team's role.`,
							h(
								"div.box",
								{},
								(a.teams || []).length
									? a.teams.map((t) =>
											h(
												"div.box-row",
												{},
												h("span.avatar.md.square", { style: { background: "var(--bg-sunken)", color: "var(--fg-muted)" } }, icon("team", "sm")),
												h("div.grow", {}, h("b", {}, t.name), h("div.small.muted", {}, fmt.plural(t.members, "member"))),
												roleSel(t.role, async (v) => {
													await api.put(R(`/teams/${encodeURIComponent(t.name)}`), { role: v });
													toast(`${t.name} is now ${v}`);
												}),
												removeBtn("team " + t.name, () => api.del(R(`/teams/${encodeURIComponent(t.name)}`))),
											),
										)
									: h("div.box-empty.small", {}, "No team has access yet."),
							),
							teams.length ? h("div.row", {}, pick, trole, addT) : h("span.small.muted", {}, h("a", { href: `/${repo.owner}?tab=teams` }, "Create a team"), " to grant it access."),
						);
					})()
				: null;
			main.replaceChildren(
				panel("Collaborators", "People with access besides the owners. Read lets them clone and review; write lets them push and land; admin lets them change these settings.", h("div.box", {}, rows.length ? rows : h("div.box-empty.small", {}, "Only the owner has access.")), h("div.row.grow", {}, who, role, add)),
				teamBox,
			);
		};
		await draw();
	},

	// ---- reviews ----
	async reviews({ R, main }) {
		const s = await api.get(R("/review/settings"));
		const approvals = input(String(s.approvals), { type: "number", min: 0, max: 10, style: { width: "100px" } });
		let checks = [...(s.checks || [])];
		const chips = h("div.row", { style: { "flex-wrap": "wrap", gap: "6px" } });
		const drawChips = () =>
			chips.replaceChildren(
				...checks.map((c) => h("span.chip", {}, icon("checkCircle", "sm"), c, h("button.btn.icon.sm.ghost", { type: "button", title: "Remove", onclick: () => ((checks = checks.filter((x) => x !== c)), drawChips()) }, icon("x", "sm")))),
				checks.length ? null : h("span.small.muted", {}, "No required checks."),
			);
		drawChips();
		const newCheck = input("", { placeholder: "check name, like ci/test", style: { "max-width": "260px" } });
		const addCheck = btn("Add", { icon: "plus" });
		const doAdd = () => {
			const v = newCheck.value.trim();
			if (v && !checks.includes(v)) checks.push(v);
			newCheck.value = "";
			drawChips();
		};
		addCheck.onclick = doAdd;
		newCheck.onkeydown = (e) => e.key === "Enter" && (e.preventDefault(), doAdd());
		main.replaceChildren(
			panel(
				"Reviews",
				"What a change needs before it can land on the default branch.",
				h("div", {}, field("Approvals needed", approvals, "Approvals from people with write access other than the change's author; each person's newest vote counts. A request for changes blocks landing until it is withdrawn."), field("Required checks", h("div.col", { style: { gap: "10px" } }, chips, h("div.row", {}, newCheck, addCheck)), "Each must report success on the newest version.")),
				saveButton(() => api.put(R("/review/settings"), { approvals: Number(approvals.value) || 0, checks })),
			),
		);
	},

	// ---- webhooks ----
	async webhooks(env) {
		const { R, main } = env;
		const list = (await api.get(R("/hooks"))).hooks || [];
		const add = btn("Add webhook", { primary: true, icon: "plus" });
		add.onclick = () => hookDialog(env, null);
		main.replaceChildren(
			panel(
				"Webhooks",
				"tinhub POSTs a signed JSON body to each URL for the events it subscribes to, and retries failures with backoff.",
				h(
					"div.box",
					{},
					list.length
						? list.map((x) =>
								h(
									"div.box-row",
									{},
									h("span", { class: x.active ? "green" : "faint" }, icon(x.active ? "checkCircle" : "pause", "sm")),
									h("div.grow", { style: { "min-width": "0" } }, h("div.mono.small.ellipsis", {}, x.url), h("div.tiny.muted", {}, x.kinds.length ? x.kinds.join(", ") : "every event", " · added ", time(x.created_at))),
									btn("Deliveries", { sm: true, onclick: () => deliveries(env, x) }),
									btn("Edit", { sm: true, icon: "edit", onclick: () => hookDialog(env, x) }),
								),
							)
						: empty("webhook", "No webhooks", "Add one to tell CI or chat about pushes and reviews."),
				),
				add,
			),
		);
	},

	// ---- runner ----
	async runner({ repo, R, main }) {
		let s;
		try {
			s = await api.get(R("/runner"));
		} catch (err) {
			if (err.status !== 403) throw err;
			s = null;
		}
		const entry = input(s ? s.entry : "main.tin", { class: "input mono", placeholder: "main.tin" });
		const sample = input(String(s ? s.sample : 20), { type: "number", min: 1, max: 1000, style: { width: "100px" } });
		const envs = textarea(s ? s.env.join("\n") : "", { rows: 4, class: "textarea mono", placeholder: "NAME=value, one a line" });
		main.replaceChildren(
			panel(
				"Behaviour runner",
				"Each review's behaviour run builds the change and its base, replays a sample of recorded production requests against both in a sandbox, and groups what changed.",
				h("div", {}, field("Entry", entry, "The program to build, from the repository's root."), field("Sample", sample, "How many recorded capsules each run replays."), field("Environment", envs, "Given to both builds. Don't put secrets here: anyone who can read the repository sees run results.")),
				saveButton(() => api.put(R("/runner"), { entry: entry.value.trim(), sample: Number(sample.value) || 20, env: envs.value.split("\n").map((l) => l.trim()).filter(Boolean) })),
			),
			repo.owner_kind === "org" ? callout("info", "Runs need the org to opt in: ", h("a", { href: `/${repo.owner}?tab=settings` }, `${repo.owner}'s settings`), ", Runner.") : callout("info", "Behaviour runs read capsules sealed for the runner's key, which an org opts into. Personal repositories replay locally with ", h("code", {}, "tit replay"), "."),
		);
	},

	// ---- replay ----
	async replay({ repo, R, main }) {
		const ret = await api.get(R("/replay/retention"));
		const days = input(String(ret.days), { type: "number", min: 1, max: 3650, style: { width: "100px" } });
		const who = userInput();
		const give = btn("Grant", { primary: true, icon: "plus" });
		const drawGrants = async () => {
			const g = await api.get(R("/replay/grants"));
			grantBox.replaceChildren(
				...(g.users || []).map((u) => {
					const b = btn("", { sm: true, ghost: true, icon: "x", title: "Take the grant away" });
					b.onclick = () =>
						busy(b, async () => {
							await api.del(R(`/replay/grants/${encodeURIComponent(u)}`));
							await drawGrants();
						}).catch(() => {});
					return h("div.box-row", {}, avatar(u, "md"), h("a.grow", { href: "/" + u }, h("b", {}, u)), b);
				}),
				...(g.teams || []).map((t) => h("div.box-row", {}, icon("team"), h("b.grow", {}, t), h("span.small.muted", {}, "team"))),
				(g.users || []).length + (g.teams || []).length ? null : h("div.box-empty.small", {}, "Only admins can read capsules."),
			);
		};
		const grantBox = h("div.box");
		give.onclick = () =>
			busy(give, async () => {
				const n = who.input.value.trim();
				if (!n) return;
				await api.put(R(`/replay/grants/${encodeURIComponent(n)}`));
				who.input.value = "";
				toast(`${n} can read capsules`);
				await drawGrants();
			}).catch(() => {});
		main.replaceChildren(
			panel("Retention", "Capsules are deleted this many days after they were sealed. Failure groups keep their counts.", field("Days", days), saveButton(() => api.put(R("/replay/retention"), { days: Number(days.value) || 30 }))),
			panel("Who can read capsules", "A capsule holds a real production request, so reading one needs the replay permission: admins have it, others need a grant. Write access only lets a server upload them.", grantBox, h("div.row.grow", {}, who, give)),
		);
		await drawGrants();
	},

	// ---- mirror ----
	async mirror({ repo, R, main }) {
		let m = null;
		try {
			m = await api.get(R("/mirror"));
		} catch (err) {
			if (err.status !== 404) throw err;
		}
		const url = input(m ? m.url : "", { class: "input mono", placeholder: "https://github.com/you/repo.git" });
		const token = input("", { type: "password", autocomplete: "new-password", placeholder: m && m.has_token ? "•••••• (kept unless you type a new one)" : "a token with push access" });
		const active = checkbox("Push to the mirror after every push here", m ? m.active : true);
		const stop = m ? btn("Remove mirror", { danger: true, icon: "trash" }) : null;
		if (stop)
			stop.onclick = () =>
				busy(stop, async () => {
					if (!(await confirm({ title: "Remove the mirror?", message: "tinhub stops pushing there and forgets the token.", confirmLabel: "Remove", danger: true }))) return;
					await api.del(R("/mirror"));
					toast("Mirror removed");
					await PAGES.mirror({ repo, R, main });
				}).catch(() => {});
		main.replaceChildren(
			panel(
				"Push mirror",
				"Keep a copy of the branches and tags on another git host. tinhub pushes after each push here.",
				h(
					"div",
					{},
					m ? h("div.mb-4", {}, m.last_error ? callout("error", h("b", {}, "The last mirror push failed: "), m.last_error) : m.mirrored_at ? callout("ok", "Mirrored ", time(m.mirrored_at), ".") : callout("info", "Not mirrored yet.")) : null,
					field("Remote URL", url),
					field("Token", token, "Stored encrypted; never shown again."),
					h("div", { style: { "margin-top": "14px" } }, active),
				),
				[
					saveButton(async () => {
						await api.put(R("/mirror"), { url: url.value.trim(), token: token.value, active: active.input.checked, keep_token: !token.value });
						await PAGES.mirror({ repo, R, main });
					}),
					h("span.spacer"),
					stop,
				],
			),
		);
	},

	// ---- audit ----
	async audit({ R, main }) {
		const rows = h("tbody");
		const box = h("div.box", {}, h("table.table", {}, h("thead", {}, h("tr", {}, ["When", "Who", "What", "Detail"].map((t) => h("th", {}, t)))), rows));
		main.replaceChildren(panel("Audit log", "Every change to the repository's settings and access, newest first.", box));
		let cursor = "";
		const load = async () => {
			const r = await api.get(R("/audit"), { cursor, limit: 50 });
			for (const e of r.entries || []) rows.append(auditRow(e));
			cursor = r.next || "";
			if (!rows.childNodes.length) box.replaceChildren(empty("scroll", "Nothing yet", "Settings changes show up here."));
			return Boolean(cursor);
		};
		if (await load()) box.after(more(load));
	},
};

// auditRow is one audit_log row. The log keeps ids; the server names what they name now (target_name, user_name,
// team_name), shown in their place. target shows what the row is about, for the site's log.
export function auditRow(e, { ip = false, target = false } = {}) {
	const named = { user: e.user_name, team: e.team_name };
	const chip = (k, v) => {
		const name = named[k];
		const text = name || (typeof v === "object" ? JSON.stringify(v) : v);
		return h("span.chip", { style: { "margin-right": "4px" }, title: name ? `${k} ${v}` : null }, `${k}: `, name ? h("a", { href: "/" + name.split("/")[0] }, text) : text);
	};
	const d = e.detail && typeof e.detail === "object" ? Object.entries(e.detail).map(([k, v]) => chip(k, v)) : null;
	const what = target && e.target_name ? h("div.small", {}, h("a", { href: "/" + (e.target_kind === "team" ? e.target_name.split("/")[0] : e.target_name) }, e.target_name)) : null;
	return h("tr", {}, h("td.small.muted", { style: { "white-space": "nowrap" } }, time(e.at)), h("td", {}, e.actor ? h("a", { href: "/" + e.actor }, e.actor) : h("span.muted", {}, "system")), h("td", {}, h("span.mono.small", {}, e.action), what, ip && e.ip ? h("div.tiny.muted", {}, e.ip) : null), h("td.small", {}, d));
}

// hookDialog adds a webhook (hook null) or edits one.
function hookDialog(env, hook) {
	const { R } = env;
	const url = input(hook ? hook.url : "", { class: "input mono", placeholder: "https://ci.example.com/hooks/tinhub", type: "url" });
	const secret = input("", { type: "password", autocomplete: "new-password", placeholder: hook ? "unchanged" : "generated when empty" });
	const kinds = new Set(hook ? hook.kinds : []);
	const every = checkbox("Every event", !kinds.size);
	const boxes = HOOK_KINDS.map((k) => {
		const c = checkbox(k, kinds.has(k));
		c.input.onchange = () => {
			if (c.input.checked) every.input.checked = false;
		};
		return [k, c];
	});
	every.input.onchange = () => {
		if (every.input.checked) for (const [, c] of boxes) c.input.checked = false;
	};
	const active = checkbox("Active", hook ? hook.active : true);
	const save = btn(hook ? "Save" : "Add webhook", { primary: true });
	const del = hook ? btn("Delete", { danger: true, icon: "trash" }) : null;
	const d = dialog({
		title: hook ? "Edit webhook" : "Add a webhook",
		body: h(
			"div",
			{},
			field("Payload URL", url),
			field("Secret", secret, "Signs each body: the X-Tinhub-Signature header is its HMAC-SHA256."),
			field("Events", h("div.col", { style: { gap: "6px" } }, every, h("div", { style: { display: "grid", "grid-template-columns": "1fr 1fr", gap: "6px", "margin-top": "4px" } }, boxes.map(([, c]) => c)))),
			h("div", { style: { "margin-top": "14px" } }, active),
		),
		actions: [del, h("span.spacer"), btn("Cancel", { onclick: () => d.close() }), save].filter(Boolean),
	});
	save.onclick = () =>
		busy(save, async () => {
			const body = { url: url.value.trim(), kinds: every.input.checked ? [] : boxes.filter(([, c]) => c.input.checked).map(([k]) => k), active: active.input.checked };
			// An empty secret keeps the hook's own (or, for a new hook, has the server make one).
			if (secret.value) body.secret = secret.value;
			if (hook) await api.patch(R(`/hooks/${hook.id}`), body);
			else {
				const r = await api.post(R("/hooks"), body);
				if (!secret.value && r.secret) {
					d.close();
					const done = btn("Done", { primary: true });
					const shown = dialog({ title: "Webhook added", body: h("div", {}, h("p", {}, "Its secret, shown only now:"), h("div.row", {}, h("code.grow", { style: { "word-break": "break-all" } }, r.secret), copyButton(r.secret))), actions: [done] });
					done.onclick = () => shown.close();
					await PAGES.webhooks(env);
					return;
				}
			}
			d.close();
			toast(hook ? "Webhook saved" : "Webhook added");
			await PAGES.webhooks(env);
		}).catch(() => {});
	if (del)
		del.onclick = () =>
			busy(del, async () => {
				if (!(await confirm({ title: "Delete this webhook?", message: hook.url, confirmLabel: "Delete", danger: true }))) return;
				await api.del(R(`/hooks/${hook.id}`));
				d.close();
				toast("Webhook deleted");
				await PAGES.webhooks(env);
			}).catch(() => {});
}

async function deliveries(env, hook) {
	const rows = h("tbody", {}, h("tr", {}, h("td", { colspan: 5 }, skeleton(3))));
	const d = dialog({ title: "Deliveries", wide: true, body: h("div", {}, h("p.small.muted.mono", {}, hook.url), h("div.box", {}, h("table.table", {}, h("thead", {}, h("tr", {}, ["Event", "Attempt", "Status", "Took", "When"].map((t) => h("th", {}, t)))), rows))), actions: [btn("Close", { onclick: () => d.close() })] });
	try {
		const r = await api.get(env.R(`/hooks/${hook.id}/deliveries`), { limit: 50 });
		const list = r.deliveries || [];
		rows.replaceChildren(
			...list.map((x) =>
				h(
					"tr",
					{},
					h("td.mono.small", {}, "#" + x.event),
					h("td", {}, String(x.attempt)),
					h("td", {}, x.status ? badge(String(x.status), x.status < 300 ? "green" : "red") : badge("failed", "red"), x.error ? h("div.tiny.red", {}, x.error) : null),
					h("td.small", {}, fmt.duration(x.duration_ms)),
					h("td.small.muted", {}, time(x.created_at)),
				),
			),
		);
		if (!list.length) rows.replaceChildren(h("tr", {}, h("td.muted.center", { colspan: 5 }, "No deliveries yet.")));
	} catch (err) {
		rows.replaceChildren(h("tr", {}, h("td", { colspan: 5 }, errorBox(err))));
	}
}
