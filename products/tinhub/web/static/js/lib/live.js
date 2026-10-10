// Live updates over the websocket /api/v1/live (notify/live.tin): follow topics ("repo:o/n", "change:o/n/c") and get
// their events. One socket for the page; it reconnects with backoff and follows its topics again.

const listeners = new Map(); // topic -> Set(fn)
const status = new Set();
let ws = null;
let open = false;
let retry = 0;
let timer = 0;

function url() {
	const proto = location.protocol === "https:" ? "wss:" : "ws:";
	const topics = [...listeners.keys()].join(",");
	return `${proto}//${location.host}/api/v1/live${topics ? "?topics=" + encodeURIComponent(topics) : ""}`;
}

function setOpen(v) {
	open = v;
	for (const fn of status) fn(v);
}

function connect() {
	clearTimeout(timer);
	if (ws || listeners.size === 0 || typeof WebSocket === "undefined") return;
	let sock;
	try {
		sock = new WebSocket(url());
	} catch {
		schedule();
		return;
	}
	ws = sock;
	sock.onopen = () => {
		retry = 0;
		setOpen(true);
	};
	sock.onmessage = (e) => {
		let m;
		try {
			m = JSON.parse(e.data);
		} catch {
			return;
		}
		if (m.op !== "event") return;
		const fns = listeners.get(m.topic);
		if (fns) for (const fn of [...fns]) fn(m);
	};
	sock.onclose = () => {
		if (ws === sock) ws = null;
		setOpen(false);
		schedule();
	};
	sock.onerror = () => {};
}

function schedule() {
	if (listeners.size === 0) return;
	clearTimeout(timer);
	const wait = Math.min(30000, 1000 * 2 ** retry) + Math.random() * 500;
	retry++;
	timer = setTimeout(connect, wait);
}

function send(op, topic) {
	if (ws && ws.readyState === 1) ws.send(JSON.stringify({ op, topic }));
}

// follow calls fn with each event of topic until the returned function is called.
export function follow(topic, fn) {
	let set = listeners.get(topic);
	const fresh = !set;
	if (!set) {
		set = new Set();
		listeners.set(topic, set);
	}
	set.add(fn);
	if (!ws) connect();
	else if (fresh) send("subscribe", topic);
	return () => {
		const s = listeners.get(topic);
		if (!s) return;
		s.delete(fn);
		if (s.size === 0) {
			listeners.delete(topic);
			send("unsubscribe", topic);
		}
	};
}

// onStatus calls fn(open) whenever the socket opens or closes.
export function onStatus(fn) {
	status.add(fn);
	fn(open);
	return () => status.delete(fn);
}

export function isOpen() {
	return open;
}
