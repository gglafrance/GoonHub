<script setup lang="ts">
// Chat text: server emotes (`:name:`), links (http/https, new tab, no referrer) and text, as
// text nodes, images and anchors: never as HTML. Shared by the Goon2gether chat and the
// messages. A text made of emotes only shows them bigger.
const props = withDefaults(defineProps<{ text: string; links?: boolean }>(), { links: true });

const { byName } = useTogetherEmotes();
const tokens = computed(() =>
    props.links
        ? tokenizeMessage(props.text, byName.value)
        : tokenizeChat(props.text, byName.value),
);
const big = computed(() => isEmoteOnly(tokens.value));
</script>

<template>
    <span class="break-words" :class="{ 'inline-block py-0.5': big }">
        <template v-for="(t, i) in tokens" :key="i">
            <img
                v-if="t.kind === 'emote'"
                :src="t.url"
                :alt="`:${t.name}:`"
                :title="`:${t.name}:`"
                class="inline-block object-contain align-middle"
                :class="big ? 'h-8 w-8' : 'h-5 w-5'"
            />
            <a
                v-else-if="t.kind === 'link'"
                :href="t.href"
                target="_blank"
                rel="noopener noreferrer nofollow"
                class="text-lava/90 hover:text-lava break-all underline decoration-white/20
                    underline-offset-2"
                >{{ t.text }}</a
            >
            <span v-else class="whitespace-pre-wrap">{{ t.text }}</span>
        </template>
    </span>
</template>
