const DEFAULT_NOTIFICATION = {
	title: "keep. 💌",
	body: "You have a new notification.",
	url: "/"
};

self.addEventListener("install", () => {
	self.skipWaiting();
});

self.addEventListener("activate", event => {
	event.waitUntil(self.clients.claim());
});

self.addEventListener("push", event => {
	let notification = { ...DEFAULT_NOTIFICATION };

	if (event.data) {
		try {
			const payload = event.data.json();

			notification = {
				title: payload.title || DEFAULT_NOTIFICATION.title,
				body: payload.body || DEFAULT_NOTIFICATION.body,
				url: payload.url || DEFAULT_NOTIFICATION.url
			};
		} catch {
			notification.body = event.data.text() || DEFAULT_NOTIFICATION.body;
		}
	}

	event.waitUntil(
		self.registration.showNotification(notification.title, {
			body: notification.body,
			icon: "/static/sunflower.png",
			badge: "/static/sunflower.png",
			data: {
				url: notification.url
			}
		})
	);
});

self.addEventListener("notificationclick", event => {
	event.notification.close();

	const targetURL = event.notification.data?.url || "/";

	event.waitUntil(
		self.clients
			.matchAll({
				type: "window",
				includeUncontrolled: true
			})
			.then(windowClients => {
				for (const client of windowClients) {
					if ("navigate" in client) {
						return client.navigate(targetURL).then(() => client.focus());
					}
				}

				return self.clients.openWindow(targetURL);
			})
	);
});