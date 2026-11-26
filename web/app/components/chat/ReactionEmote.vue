<script setup lang="ts">
// A reaction key: the image of a server emote (`:name:`), or the emoji. An emote that no
// longer exists shows as its `:name:` text.
const props = defineProps<{ value: string }>();

const { byName, load } = useTogetherEmotes();
onMounted(() => load());

const emote = computed(() => {
    const match = /^:([A-Za-z0-9_]{2,32}):$/.exec(props.value);
    return match ? (byName.value.get(match[1]!.toLowerCase()) ?? null) : null;
});
</script>

<template>
    <img
        v-if="emote"
        :src="emote.url"
        :alt="value"
        class="inline-block h-4 w-4 shrink-0 object-contain"
    />
    <span v-else class="leading-none">{{ value }}</span>
</template>
