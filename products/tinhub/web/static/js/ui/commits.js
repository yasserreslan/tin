// Commits read once per page load: their title, message and author, for lists of changes and reviews.

import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import { h } from "../lib/dom.js";

const cache = new Map();

export function commitOf(repo, id) {
	if (!id) return Promise.resolve(null);
	const key = `${repo.owner}/${repo.name}/${id}`;
	if (!cache.has(key)) {
		const p = api.get(api.R(repo.owner, repo.name)(`/commits/${id}`));
		cache.set(key, p);
		p.catch(() => cache.delete(key));
	}
	return cache.get(key);
}

// title is a span that fills in with the commit's title.
export function title(repo, id, fallback = "") {
	const el = h("span", {}, fallback || " ");
	commitOf(repo, id).then(
		(c) => {
			if (c) el.textContent = fmt.title(c.message) || "(no message)";
		},
		() => {},
	);
	return el;
}
