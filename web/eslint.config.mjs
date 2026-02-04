// @ts-check
import withNuxt from './.nuxt/eslint.config.mjs';
import babelParser from '@babel/eslint-parser';

export default withNuxt([
    {
        ignores: [],
        files: ['**/*.js'],
        languageOptions: {
            parser: babelParser,
            parserOptions: {
                requireConfigFile: false,
                babelOptions: {
                    babelrc: false,
                    configFile: false,
                },
            },
        },
    },
    {
        rules: {
            'vue/no-multiple-template-root': 'off',
            'vue/no-v-html': 'off',
            'vue/no-mutating-props': 'off',
            'vue/one-component-per-file': 'off',
            'vue/html-self-closing': 'off',
            'vue/multi-word-component-names': 'off',
            'no-useless-escape': 'off',
        },
    },
]);
