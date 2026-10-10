// Accepting an invite: /join?code=…&email=… shows the command that registers the visitor's key with it.

import { h } from "../lib/dom.js";
import { mark } from "../lib/icons.js";
import * as store from "../lib/store.js";
import { copyButton, btn } from "../ui/kit.js";

export async function render(ctx) {
	ctx.title("Join");
	const code = ctx.query.code || "<invite code>";
	const add = `tit key add ${store.site.url} ${code}`;
	const step = (title, ...body) => h("div.step", {}, h("div.grow", {}, h("div.strong", {}, title), h("div", { style: { "margin-top": "8px" } }, body)));
	ctx.main.replaceChildren(
		h(
			"div.auth",
			{},
			h(
				"div.card.auth-card",
				{},
				h(
					"div.card-body",
					{},
					h("div.center", { style: { "margin-bottom": "24px" } }, mark(44), h("h1", { style: { "margin-top": "14px", "font-size": "22px" } }, "You're invited to tinhub"), ctx.query.email ? h("p.muted", { style: { "margin-top": "6px" } }, "The invite is for ", h("b", {}, ctx.query.email), ".") : null),
					h(
						"div.steps",
						{},
						step("Install tit, and make your key", h("div.copy-line", {}, h("code", {}, "tit key"), copyButton("tit key"))),
						step("Register the key with your invite", h("div.copy-line", {}, h("code", {}, add), copyButton(add)), h("p.hint", { style: { "margin-top": "6px" } }, "The code works once, for the email it was made for.")),
						step("Sign in", btn("Sign in with your key", { primary: true, icon: "login", href: "/login" })),
					),
				),
			),
		),
	);
}
