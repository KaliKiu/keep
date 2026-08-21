self.addEventListener("install", () => {
	console.log("keep. service worker installed");
});

self.addEventListener("activate", () => {
	console.log("keep. service worker activated");
});