// https://nuxt.com/docs/api/configuration/nuxt-config
import tailwindcss from '@tailwindcss/vite';

export default defineNuxtConfig({
    compatibilityDate: '2025-07-15',
    devtools: { enabled: true },

    app: {
        head: {
            title: 'GoonHub',
            titleTemplate: '%s - GoonHub',
        },
    },

    vue: {
        compilerOptions: {
            isCustomElement: (tag) => 
                tag.startsWith('media-') || 
                tag === 'videojs-video' || 
                tag === 'media-theme',
        },
    },

    modules: [
        '@nuxt/eslint',
        '@pinia/nuxt',
        '@nuxt/icon',
        '@pinia-plugin-persistedstate/nuxt',
    ],

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
            },
        },
    },
});
