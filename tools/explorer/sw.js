// PgArachne Explorer - Service Worker
// Provides offline caching for PWA support

const CACHE_NAME = 'pgarachne-explorer-v1';
const ASSETS = [
	'./',
	'./index.html',
	'./icon-192.png',
	'./icon-512.png'
];

// Install: cache core assets
self.addEventListener('install', event => {
	event.waitUntil(
		caches.open(CACHE_NAME).then(cache => cache.addAll(ASSETS))
	);
	self.skipWaiting();
});

// Activate: clean up old caches
self.addEventListener('activate', event => {
	event.waitUntil(
		caches.keys().then(keys =>
			Promise.all(keys.filter(k => k !== CACHE_NAME).map(k => caches.delete(k)))
		)
	);
	self.clients.claim();
});

// Fetch: network first, fallback to cache (API calls always go to network)
self.addEventListener('fetch', event => {
	// Don't cache API requests
	if (event.request.method !== 'GET') return;

	event.respondWith(
		fetch(event.request)
			.then(response => {
				// Cache updated version
				const clone = response.clone();
				caches.open(CACHE_NAME).then(cache => cache.put(event.request, clone));
				return response;
			})
			.catch(() => caches.match(event.request))
	);
});
