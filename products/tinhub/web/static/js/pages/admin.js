// Site administration (site admins): invites, and the audit log of the whole site. Org owners can invite into their
// orgs here too.

import { h, debounce } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as store from "../lib/store.js";
import { navigate } from "../lib/router.js";
import { sidenav, panel, input, select, checkbox } from "../ui/forms.js";
import { auditRow } from "./reposettings.js";
import { badge, btn, busy, callout, copyButton, empty, errorBox, field, more, skeleton, time, toast } from "../ui/kit.js";

export async function render(ctx) {
	const me = store.session.user;
	if (!me) return navigate("/login?next=" + encodeURIComponent(location.pathname), { replace: true });
	const owned = (me.orgs || []).filter((o) => o.role === "owner");
	const sections = [{ key: "invites", label: "Invites", icon: "mail" }, me.site_admin ? { key: "audit", label: "Audit log", icon: "scroll" } : null].filter(Boolean);
	const section = ctx.params.section || "invites";
	if (!me.site_admin && !owned.length) {
		ctx.main.append(h("div.container.page", {}, h("div.box", {}, empty("shield", "Nothing to administer", "Site admins invite people and read the audit log here; org owners invite people into their orgs."))));
		return;
	}
	if (!sections.some((s) => s.key === section)) return ctx.notFound();
	ctx.title(me.site_admin ? "Admin" : "Invites");
	const main = h("div", {}, skeleton(5));
	ctx.main.append(
		h(
			"div.container.page",
			{},
			h("div.page-head", {}, h("div", {}, h("h1", {}, me.site_admin ? "Site admin" : "Invites"), h("p.sub", {}, location.host))),
			h("div.layout-sidebar", {}, sidenav(sections.map((s) => ({ ...s, href: "/admin/" + s.key })), section), main),
		),
	);
	try {
		if (section === "audit") await audit(ctx, main);
		else await invites(ctx, me, owned, main);
	} catch (err) {
		main.replaceChildren(errorBox(err));
	}
}

async function invites(ctx, me, owned, main) {
	const email = input("", { type: "email", placeholder: "person@example.com", autocomplete: "off" });
	// the name with the display name: two orgs can share a display name
	const orgs = [...(me.site_admin ? [["", "No org"]] : []), ...owned.map((o) => [o.name, o.display && o.display !== o.name ? `${o.display} (${o.name})` : o.name])];
	const org = select(orgs, me.site_admin ? "" : owned[0].name);
	const admin = me.site_admin ? checkbox("Make them a site admin", false, "Site admins can read every repository and this page.") : null;
	const result = h("div");
	const send = btn("Create invite", { primary: true, icon: "send", type: "submit" });
	const form = h("form", {}, h("div.form-grid", {}, field("Email", email), field("Into org", org)), admin ? h("div.mt-4", {}, admin) : null, h("div.form-actions", {}, send), result);
	const pending = h("div.box", {}, skeleton(3));
	const loadPending = async () => {
		const r = await api.get("/invites");
		const list = r.invites || [];
		pending.replaceChildren(
			// an invite with no maker was made on the server (tinhub invite), the audit log's "system"
			...(list.length
				? list.map((x) =>
						h(
							"div.box-row",
							{},
							icon("mail", "sm"),
							h("div.grow", {}, h("b", {}, x.email), h("div.small.muted", {}, x.org ? ["into ", h("a", { href: "/" + x.org }, x.org), " · "] : null, x.created_by ? ["by ", h("a", { href: "/" + x.created_by }, x.created_by)] : "made on the server", " · ", time(x.created_at))),
							x.site_admin ? badge("site admin", "purple") : null,
							h("span.small.muted", {}, "expires ", time(x.expires_at)),
						),
					)
				: [empty("mail", "No pending invites", "Invites someone has used or that expired drop off this list.")]),
		);
	};
	form.onsubmit = (e) => {
		e.preventDefault();
		busy(send, async () => {
			const v = email.value.trim();
			if (!v) return;
			const r = await api.post("/invites", { email: v, org: org.value, site_admin: admin ? admin.input.checked : false });
			const cmd = r.command || `tit key add ${location.origin} ${r.code}`;
			result.replaceChildren(
				h(
					"div.mt-4",
					{},
					callout("ok", h("div", {}, h("b", {}, `Invite for ${r.email} created.`), " Send them this command; it works once, until ", time(r.expires), ".")),
					h("div.terminal-line.mt-2", {}, h("code.grow", {}, cmd), copyButton(cmd)),
					h("p.small.muted.mt-2", {}, "Or a link to the web page that explains it: ", h("a", { href: `/join?code=${encodeURIComponent(r.code)}&email=${encodeURIComponent(r.email)}` }, `${location.origin}/join?code=…`), " ", copyButton(`${location.origin}/join?code=${encodeURIComponent(r.code)}&email=${encodeURIComponent(r.email)}`, { title: "Copy the link" })),
				),
			);
			email.value = "";
			await loadPending();
		}).catch(() => {});
	};
	main.replaceChildren(panel("Invite someone", "tinhub has no passwords: a new person installs tit, makes a key, and adds it with a one-use invite.", form), panel("Pending invites", null, pending));
	await loadPending();
}

async function audit(ctx, main) {
	const filter = input(ctx.query.action || "", { placeholder: "filter by action, like repo. or org.member", style: { "max-width": "320px" } });
	const rows = h("tbody");
	const table = h("div.box", {}, h("table.table", {}, h("thead", {}, h("tr", {}, ["When", "Who", "What", "Detail"].map((t) => h("th", {}, t)))), rows));
	const moreBox = h("div");
	let cursor = "";
	const load = async (reset) => {
		if (reset) {
			cursor = "";
			rows.replaceChildren();
		}
		const r = await api.get("/admin/audit", { cursor, limit: 50, action: filter.value.trim() });
		for (const e of r.entries || []) rows.append(auditRow(e, { ip: true, target: true }));
		cursor = r.next || "";
		if (!rows.childNodes.length) rows.append(h("tr", {}, h("td.muted.center", { colspan: 4 }, "Nothing matches.")));
		if (reset) moreBox.replaceChildren(cursor ? more(() => load(false)) : "");
		return Boolean(cursor);
	};
	filter.oninput = debounce(() => load(true).catch((err) => toast(err.message, "error")), 300);
	main.replaceChildren(panel("Audit log", "Every change to accounts, orgs, repositories and access on this site, with the address it came from.", h("div", {}, h("div.mb-4", {}, filter), table, moreBox)));
	await load(true);
}
