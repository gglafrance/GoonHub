// https://nuxt.com/docs/api/configuration/nuxt-config
import tailwindcss from '@tailwindcss/vite';

export default defineNuxtConfig({
    compatibilityDate: '2025-07-15',
    devtools: { enabled: true },

    app: {
        head: {
            title: 'GoonHub',
            titleTemplate: '%s - GoonHub',
            link: [
                { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
                { rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' },
                { rel: 'apple-touch-icon', href: '/apple-touch-icon.png' },
            ],
            meta: [
                { name: 'description', content: 'GoonHub - Your personal video library' },
                { property: 'og:site_name', content: 'GoonHub' },
                { property: 'og:type', content: 'website' },
                { name: 'theme-color', content: '#0F0F0F' },
            ],
        },
    },

    vue: {
        compilerOptions: {
            isCustomElement: (tag) =>
                tag.startsWith('media-') || tag === 'videojs-video' || tag === 'media-theme',
        },
    },

    modules: ['@nuxt/eslint', '@pinia/nuxt', '@nuxt/icon', '@pinia-plugin-persistedstate/nuxt'],

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
