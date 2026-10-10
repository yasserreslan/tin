// Your account: profile, keys, what you follow and how tinhub looks.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import * as store from "../lib/store.js";
import { navigate } from "../lib/router.js";
import { sidenav, panel, input, saveButton } from "../ui/forms.js";
import { avatar, badge, btn, busy, confirm, copyButton, empty, errorBox, field, kv, repoLink, time, toast, loginHref } from "../ui/kit.js";

const SECTIONS = [
	{ key: "profile", label: "Profile", icon: "user" },
	{ key: "keys", label: "Keys", icon: "key" },
	{ key: "following", label: "Following", icon: "bell" },
	{ key: "appearance", label: "Appearance", icon: "sun" },
];

export async function render(ctx) {
	const me = store.session.user;
	if (!me) return navigate(loginHref(), { replace: true });
	const section = ctx.params.section || "profile";
	if (!SECTIONS.some((s) => s.key === section)) return ctx.notFound();
	ctx.title("Settings");
	const main = h("div");
	const head = (u) => h("div.page-head", {}, h("div.row", { style: { gap: "14px" } }, avatar(u.display || u.name, "lg"), h("div", {}, h("h1", {}, u.display || u.name), h("p.sub", { style: { margin: "2px 0 0" } }, h("a", { href: "/" + u.name }, u.name), " · your account"))));
	let top = head(me);
	// A saved display name shows in the head at once.
	ctx.cleanup(
		store.onUser((u) => {
			if (!u) return;
			const next = head(u);
			top.replaceWith(next);
			top = next;
		}),
	);
	ctx.main.append(h("div.container.page", {}, top, h("div.layout-sidebar", {}, sidenav(SECTIONS.map((s) => ({ ...s, href: s.key === "profile" ? "/settings" : "/settings/" + s.key })), section), main)));
	try {
		await PAGES[section](ctx, me, main);
	} catch (err) {
		main.replaceChildren(errorBox(err));
	}
}

const PAGES = {
	async profile(ctx, me, main) {
		const display = input(me.display || "", { maxlength: 100, placeholder: me.name });
		main.replaceChildren(
			panel(
				"Profile",
				null,
				h("div", {}, field("Display name", display, "Shown next to your user name on reviews, commits and your page."), h("div.mt-4", {}, kv([["User name", h("span.mono", {}, me.name)], ["Email", me.email], ["Joined", time(me.created_at)], me.site_admin ? ["Role", badge("Site admin", "purple", "shield")] : null]))),
				saveButton(async () => {
					const u = await api.patch("/user", { display: display.value.trim() });
					store.setUser({ ...me, ...u });
				}),
			),
			(me.orgs || []).length
				? panel(
						"Orgs",
						null,
						h(
							"div.box",
							{},
							me.orgs.map((o) => h("a.box-row.plain", { href: "/" + o.name }, avatar(o.display || o.name, "md", { square: true }), h("div.grow", {}, h("b", {}, o.display || o.name), h("div.small.muted", {}, o.name)), badge(o.role === "owner" ? "Owner" : "Member", o.role === "owner" ? "purple" : "outline"))),
						),
						btn("New org", { href: "/new/org", icon: "plus", sm: true }),
					)
				: panel("Orgs", "You're not in an org.", null, btn("New org", { href: "/new/org", icon: "plus", sm: true })),
		);
	},

	async keys(ctx, me, main) {
		const draw = async () => {
			const r = await api.get("/user/keys");
			const keys = r.keys || [];
			main.replaceChildren(
				panel(
					"Keys",
					"tit signs every request with one of these Ed25519 keys; signing in on the web is a request your key approves.",
					h(
						"div.box",
						{},
						keys.length
							? keys.map((k) => {
									const rm = btn("Remove", { sm: true, danger: true });
									rm.onclick = () =>
										busy(rm, async () => {
											if (!(await confirm({ title: "Remove this key?", message: `${k.fingerprint}\n\ntit on the machine holding it can no longer push or sign in.`, confirmLabel: "Remove key", danger: true }))) return;
											await api.del(`/user/keys/${k.id}`);
											toast("Key removed");
											await draw();
										}).catch(() => {});
									return h(
										"div.box-row",
										{ style: { "align-items": "flex-start", padding: "14px 16px" } },
										h("span.avatar.md.square", { style: { background: "var(--accent-soft)", color: "var(--accent)" } }, icon("key", "sm")),
										h("div.grow", { style: { "min-width": "0" } }, h("b", {}, k.name || "Ed25519 key"), h("div.mono.small.muted.ellipsis", {}, k.fingerprint), h("div.tiny.faint", { style: { "margin-top": "4px" } }, "Added ", time(k.created_at), " · ", k.last_used_at ? ["last used ", time(k.last_used_at)] : "never used")),
										copyButton(k.public_key, { title: "Copy the public key" }),
										keys.length > 1 ? rm : null,
									);
								})
							: empty("key", "No keys", ""),
					),
					h("div.small.muted", {}, "To add a key on a new machine, run ", h("code", {}, `tit key add ${location.origin} CODE`), " there with a one-use invite from a site admin."),
				),
			);
		};
		await draw();
	},

	async following(ctx, me, main) {
		const draw = async () => {
			const r = await api.get("/subscriptions");
			const subs = r.subscriptions || [];
			main.replaceChildren(
				panel(
					"Following",
					"You get mail and see activity for the repositories and changes you follow, and for the ones you can write to.",
					h(
						"div.box",
						{},
						subs.length
							? subs.map((s) => {
									const [o, n] = s.repo.split("/");
									const un = btn("Unfollow", { sm: true, icon: "bellOff" });
									un.onclick = () =>
										busy(un, async () => {
											await api.del(s.change ? `/repos/${o}/${n}/changes/${s.change}/subscription` : `/repos/${o}/${n}/subscription`);
											await draw();
										}).catch(() => {});
									return h("div.box-row", {}, icon(s.change ? "change" : "repo", "sm"), h("div.grow", {}, repoLink(s.repo, { icon: false }), s.change ? [h("span.faint", {}, " · change "), h("a.mono", { href: `/${o}/${n}/change/${s.change}` }, fmt.shortChange(s.change))] : null), un);
								})
							: empty("bell", "You don't follow anything", "Follow a repository or a change from its page."),
					),
				),
			);
		};
		await draw();
	},

	async appearance(ctx, me, main) {
		const cur = store.theme();
		const opt = (v, ic, title, text) => {
			const radio = h("input", { type: "radio", name: "theme", value: v, checked: v === cur });
			const card = h("label.radio-card", { class: v === cur ? "on" : "" }, radio, h("div", {}, h("div.row", { style: { gap: "6px" } }, icon(ic, "sm"), h("b", {}, title)), h("div.small.muted", {}, text)));
			radio.onchange = () => {
				store.setTheme(v);
				for (const c of main.querySelectorAll(".radio-card")) c.classList.toggle("on", c === card);
			};
			return card;
		};
		main.replaceChildren(panel("Theme", "Saved in this browser.", h("div.col", { style: { gap: "8px" } }, opt("system", "monitor", "System", "Follow your device's light or dark setting."), opt("light", "sun", "Light", "Always light."), opt("dark", "moon", "Dark", "Always dark."))));
	},
};
