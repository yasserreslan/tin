// Sign-in with a key (design/tit.md §18): the page asks for a login code, shows the `tit login` command to run, and
// polls until the key approves it; then it claims the session.

import { h, replace } from "../lib/dom.js";
import { icon, mark } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as store from "../lib/store.js";
import { navigate } from "../lib/router.js";
import { btn, copyButton, callout } from "../ui/kit.js";

function safeNext(next) {
	return next && next.startsWith("/") && !next.startsWith("//") && !next.startsWith("/login") ? next : "/";
}

export async function render(ctx) {
	ctx.title("Sign in");
	const next = safeNext(ctx.query.next);
	if (store.session.user) {
		navigate(next, { replace: true });
		return;
	}
	const body = h("div.card-body");
	ctx.main.replaceChildren(h("div.auth", {}, h("div.card.auth-card", {}, body)));
	let timer = 0;
	ctx.cleanup(() => clearTimeout(timer));

	const fresh = async () => {
		clearTimeout(timer);
		replace(body, h("div.loading", {}, h("span.spinner"), "Asking for a code…"));
		let a;
		try {
			a = await api.post("/tit/v1/login");
		} catch (err) {
			replace(body, callout("error", err.message), h("div.form-actions", {}, btn("Try again", { onclick: fresh })));
			return;
		}
		if (!ctx.alive()) return;
		show(a);
	};

	const show = (a) => {
		const cmd = `tit login ${store.site.url} --code ${a.code}`;
		const status = h("div.row.small.muted", {}, h("span.spinner"), "Waiting for your key to approve…");
		const step2 = h("div.step", {}, h("div.grow", {}, h("div.strong", {}, "Run this where your key lives"), h("div.copy-line", { style: { "margin-top": "8px" } }, h("code", {}, cmd), copyButton(cmd))));
		const chars = [...a.code].map((c) => (c === "-" ? h("span.dash", {}, "–") : h("span", {}, c)));
		replace(
			body,
			h("div.center", { style: { "margin-bottom": "20px" } }, mark(44), h("h1", { style: { "margin-top": "14px", "font-size": "22px" } }, "Sign in to tinhub"), h("p.muted", { style: { "margin-top": "6px" } }, "No passwords. Your tit key approves this browser.")),
			h("div.code-display", { "aria-label": "Your code " + a.code }, chars),
			h("div.steps", { style: { "margin-top": "20px" } }, step2, h("div.step", {}, h("div.grow", {}, h("div.strong", {}, "This page signs you in"), h("div", { style: { "margin-top": "6px" } }, status)))),
			h("hr"),
			h("details", {}, h("summary.small.muted", { style: { cursor: "pointer" } }, "No key registered here yet?"), h("div.small.soft", { style: { "margin-top": "10px" } }, "Ask an admin for an invite, then register your key with ", h("code", {}, `tit key add ${store.site.url} <invite code>`), ". If someone sent you an invite link, open it: it shows the exact command.")),
		);
		const expires = (a.expires || 0) * 1000;
		const poll = async () => {
			if (!ctx.alive()) return;
			if (expires && Date.now() > expires) {
				expired();
				return;
			}
			let st;
			try {
				st = await api.get(`/tit/v1/login/${encodeURIComponent(a.code)}`);
			} catch (err) {
				if (err.status === 404) {
					expired();
					return;
				}
				timer = setTimeout(poll, 3000);
				return;
			}
			if (st.state === "approved") {
				replace(status, icon("checkCircle"), " Approved by ", h("b", {}, st.email), ". Signing in…");
				status.classList.remove("muted");
				status.classList.add("green");
				step2.classList.add("done");
				try {
					await api.post(`/tit/v1/login/${encodeURIComponent(a.code)}/session`);
				} catch (err) {
					replace(status, callout("error", "The approval couldn't become a session: " + err.message));
					return;
				}
				await store.loadUser();
				navigate(next, { replace: true });
				return;
			}
			if (st.state === "expired") {
				expired();
				return;
			}
			timer = setTimeout(poll, 1500);
		};
		const expired = () => replace(status, h("span.amber", {}, icon("clock", "sm"), " This code expired. "), btn("Get a new code", { sm: true, icon: "refresh", onclick: fresh }));
		timer = setTimeout(poll, 1200);
	};
	fresh();
}
