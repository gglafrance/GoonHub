// https://nuxt.com/docs/api/configuration/nuxt-config
import tailwindcss from '@tailwindcss/vite';

export default defineNuxtConfig({
    compatibilityDate: '2025-07-15',
    devtools: { enabled: true },

    app: {
        head: {
            title: 'GoonHub',
            titleTemplate: '%s - GoonHub',
            htmlAttrs: {
                style: 'background-color:#050505',
            },
            bodyAttrs: {
                style: 'background-color:#050505',
            },
            link: [
                { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
                {
                    rel: 'preconnect',
                    href: 'https://fonts.gstatic.com',
                    crossorigin: '',
                },
                {
                    rel: 'stylesheet',
                    href: 'https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600&family=Outfit:wght@400;500;600;700&display=swap',
                },
                { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
                { rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' },
                { rel: 'apple-touch-icon', href: '/apple-touch-icon.png' },
                { rel: 'manifest', href: '/manifest.webmanifest' },
            ],
            meta: [
                { name: 'description', content: 'GoonHub - Your personal video library' },
                { property: 'og:site_name', content: 'GoonHub' },
                { property: 'og:type', content: 'website' },
                { name: 'theme-color', content: '#050505' },
                { name: 'mobile-web-app-capable', content: 'yes' },
                { name: 'apple-mobile-web-app-capable', content: 'yes' },
                { name: 'apple-mobile-web-app-status-bar-style', content: 'black-translucent' },
                { name: 'apple-mobile-web-app-title', content: 'GoonHub' },
            ],
        },
    },

    vue: {
        compilerOptions: {
            isCustomElement: (tag) => tag.startsWith('media-'),
        },
    },

    modules: [
        '@nuxt/eslint',
        '@pinia/nuxt',
        '@nuxt/icon',
        '@pinia-plugin-persistedstate/nuxt',
        '@vite-pwa/nuxt',
    ],

    pwa: {
        registerType: 'autoUpdate',
        manifest: {
            name: 'GoonHub',
            short_name: 'GoonHub',
            description: 'Your personal video library',
            theme_color: '#050505',
            background_color: '#050505',
            display: 'standalone',
            orientation: 'portrait',
            icons: [
                { src: 'pwa-192x192.png', sizes: '192x192', type: 'image/png' },
                { src: 'pwa-512x512.png', sizes: '512x512', type: 'image/png' },
                {
                    src: 'pwa-maskable-192x192.png',
                    sizes: '192x192',
                    type: 'image/png',
                    purpose: 'maskable',
                },
                {
                    src: 'pwa-maskable-512x512.png',
                    sizes: '512x512',
                    type: 'image/png',
                    purpose: 'maskable',
                },
            ],
            screenshots: [
                {
                    src: 'screenshot-wide.png',
                    sizes: '1280x720',
                    type: 'image/png',
                    form_factor: 'wide',
                },
                {
                    src: 'screenshot-narrow.png',
                    sizes: '750x1334',
                    type: 'image/png',
                    form_factor: 'narrow',
                },
            ],
        },
        workbox: {
            navigateFallback: '/',
            globPatterns: ['**/*.{js,css,html,png,svg,ico,woff2}'],
            runtimeCaching: [
                {
                    urlPattern: /^\/api\/.*/i,
                    handler: 'NetworkFirst',
                    options: {
                        cacheName: 'api-cache',
                        expiration: { maxAgeSeconds: 60 * 5 },
                    },
                },
                {
                    urlPattern: /^\/(thumbnails|sprites|actor-images|marker-thumbnails)\/.*/i,
                    handler: 'CacheFirst',
                    options: {
                        cacheName: 'media-cache',
                        expiration: { maxEntries: 500, maxAgeSeconds: 60 * 60 * 24 * 30 },
                    },
                },
            ],
        },
        devOptions: {
            enabled: true,
            type: 'module',
        },
    },

    icon: {
        clientBundle: {
            scan: true,
        },
    },

    css: ['./app/assets/css/main.css'],

    ssr: false,

    pinia: {
        storesDirs: ['./stores/**'],
    },

    nitro: {
        output: {
            publicDir: 'dist',
        },
    },

    vite: {
        plugins: [tailwindcss()],
        build: {
            rollupOptions: {
                output: {
                    manualChunks(id) {
                        if (
                            id.includes('node_modules/vue/') ||
                            id.includes('node_modules/@vue/') ||
                            id.includes('node_modules/vue-router/')
                        ) {
                            return 'vue-vendor';
                        }
                        if (
                            id.includes('node_modules/video.js/') ||
                            id.includes('node_modules/@videojs/')
                        ) {
                            return 'videojs';
                        }
                    },
                },
            },
        },
        server: {
            proxy: {
                '/api': {
                    target: 'http://localhost:8080',
                    changeOrigin: true,
                },
                '/thumbnails': {
                    target: 'http://localhost:8080',
                    changeOrigin: true,
                },
                '/sprites': {
                    target: 'http://localhost:8080',
                    changeOrigin: true,
                },
                '/vtt': {
                    target: 'http://localhost:8080',
                    changeOrigin: true,
                },
                '/actor-images': {
                    target: 'http://localhost:8080',
                    changeOrigin: true,
                },
                '/marker-thumbnails': {
                    target: 'http://localhost:8080',
                    changeOrigin: true,
                },
            },
        },
    },

    imports: {
        dirs: [
            // Scan all subdirectories of composables/
            'composables/**',
        ],
    },
});
