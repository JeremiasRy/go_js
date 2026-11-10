import { dev } from '$app/environment';
import type { Handle } from '@sveltejs/kit';

const BACKEND_URL = 'http://localhost:8000';
const SESSION_BOOTSTRAP_ID = 'go-js-session';

async function createSessionBootstrap(): Promise<string> {
	const response = await fetch(`${BACKEND_URL}/session`, { method: 'POST' });
	if (!response.ok) {
		throw new Error(`Failed to create a session: ${response.status}`);
	}

	const session = await response.json();
	const payload = JSON.stringify(session).replace(/</g, '\\u003c');
	return `<script id="${SESSION_BOOTSTRAP_ID}" type="application/json">${payload}</script>`;
}

export const handle: Handle = async ({ event, resolve }) => {
	if (
		!dev ||
		event.request.method !== 'GET' ||
		(event.url.pathname !== '/' && event.url.pathname !== '/index.html')
	) {
		return resolve(event);
	}

	const script = await createSessionBootstrap();
	const response = await resolve(event);

	if (!response.headers.get('content-type')?.includes('text/html')) {
		return response;
	}

	const html = (await response.text()).replace('</head>', `${script}</head>`);
	const headers = new Headers(response.headers);
	headers.delete('content-length');

	return new Response(html, {
		status: response.status,
		statusText: response.statusText,
		headers
	});
};
