self.addEventListener("install", () => {
	console.log("keep. service worker installed");
	self.skipWaiting();
});

self.addEventListener("activate", event => {
	console.log("keep. service worker activated");

	event.waitUntil(self.clients.claim());
});

self.addEventListener("push", event => {
	let data = {
		title: "keep. 💌",
		body: "You have a new notification.",
		url: "/"
	};

	if (event.data) {
		try {
			data = event.data.json();
		} catch {
			data.body = event.data.text();
		}
	}

	event.waitUntil(
		self.registration.showNotification(data.title || "keep. 💌", {
			body: data.body || "You have a new notification.",
			icon: "/static/sunflower.png",
			data: {
				url: data.url || "/"
			}
		})
	);
});

self.addEventListener("notificationclick", event => {
	event.notification.close();

	const url = event.notification.data?.url || "/";

	event.waitUntil(
		clients.matchAll({
			type: "window",
			includeUncontrolled: true
		}).then(windowClients => {
			for (const client of windowClients) {
				if ("focus" in client) {
					client.navigate(url);
					return client.focus();
				}
			}

			return clients.openWindow(url);
		})
	);
});