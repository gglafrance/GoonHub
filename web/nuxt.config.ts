// https://nuxt.com/docs/api/configuration/nuxt-config
import tailwindcss from '@tailwindcss/vite';

export default defineNuxtConfig({
    compatibilityDate: '2025-07-15',
    devtools: { enabled: true },

    modules: ['@nuxt/eslint', '@pinia/nuxt'],

    css: ['./app/assets/css/main.css'],

    ssr: false,

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
            },
        },
    },
});
