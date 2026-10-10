// Create an org: a shared owner for repositories, with members and teams.

import { h } from "../lib/dom.js";
import * as api from "../lib/api.js";
import * as store from "../lib/store.js";
import { navigate } from "../lib/router.js";
import { input } from "../ui/forms.js";
import { btn, busy, field } from "../ui/kit.js";

const NAME = /^[a-z0-9][a-z0-9-]{0,38}$/;

export async function render(ctx) {
	const me = store.session.user;
	if (!me) return navigate("/login?next=" + encodeURIComponent(location.pathname), { replace: true });
	ctx.title("New org");
	const name = input("", { placeholder: "acme", autocomplete: "off", spellcheck: "false" });
	const display = input("", { placeholder: "Acme Inc.", maxlength: 100 });
	const err = h("div.error-text");
	const create = btn("Create org", { primary: true, type: "submit", disabled: true });
	name.oninput = () => {
		const v = name.value.trim();
		err.textContent = v && !NAME.test(v) ? "Use lowercase letters, digits and dashes, starting with a letter or digit (at most 39)." : "";
		create.disabled = !v || Boolean(err.textContent);
	};
	const form = h("form.card", {}, h("div.card-body", {}, field("Name", name, `The org's address: ${location.host}/<name>.`), err, h("div.mt-4", {}, field("Display name", display))), h("div.card-foot.row", {}, create, h("span.small.muted", {}, "You become its first owner.")));
	form.onsubmit = (e) => {
		e.preventDefault();
		busy(create, async () => {
			await api.post("/orgs", { name: name.value.trim(), display: display.value.trim() });
			await store.loadUser();
			navigate(`/${name.value.trim()}?tab=members`);
		}).catch(() => {});
	};
	ctx.main.append(h("div.container.slim.page", {}, h("div.page-head", {}, h("div", {}, h("h1", {}, "New org"), h("p.sub", {}, "Orgs own repositories together; owners manage members, teams and access."))), form));
	setTimeout(() => name.focus(), 30);
}
