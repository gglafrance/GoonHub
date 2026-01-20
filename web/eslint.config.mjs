// @ts-check
import withNuxt from './.nuxt/eslint.config.mjs';

export default withNuxt([
    {
        ignores: [],
        parser: 'babel-eslint',
    },
    {
        rules: {
            'vue/no-multiple-template-root': 'off',
            'vue/no-v-html': 'off',
            'vue/no-mutating-props': 'off',
            'vue/one-component-per-file': 'off',
            'vue/html-self-closing': 'off',
        },
    },
]);
