// The old Next.js app registered a caching service worker at /sw.js. This
// replacement removes it: browsers check /sw.js for updates, install this
// version, which deletes every cache, unregisters itself and reloads open
// tabs once so they load the current app from the network.
self.addEventListener('install', () => self.skipWaiting());

self.addEventListener('activate', (event) => {
  event.waitUntil(
    (async () => {
      const names = await caches.keys();
      await Promise.all(names.map((name) => caches.delete(name)));
      await self.registration.unregister();
      const tabs = await self.clients.matchAll({ type: 'window' });
      tabs.forEach((tab) => tab.navigate(tab.url));
    })(),
  );
});
