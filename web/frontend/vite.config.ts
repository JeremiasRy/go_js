import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

const backend = 'http://localhost:8000';

export default defineConfig({
	plugins: [sveltekit(), tailwindcss()],
	server: {
		proxy: {
			'/session': {
				target: backend,
				changeOrigin: false
			},
			'/ws': {
				target: 'ws://localhost:8000',
				ws: true,
				changeOrigin: false
			}
		}
	}
});
