<script setup lang="ts">
// Bio of a profile: plain text with its line breaks, http(s) links made clickable (text nodes
// only, never HTML).
const props = defineProps<{ bio: string }>();
const segments = computed(() => bioSegments(props.bio));
</script>

<template>
    <p class="text-[13px] leading-relaxed break-words whitespace-pre-line text-white/80">
        <template v-for="(seg, i) in segments" :key="i">
            <a
                v-if="seg.href"
                :href="seg.href"
                target="_blank"
                rel="noopener noreferrer nofollow ugc"
                class="text-lava hover:underline"
                >{{ seg.text }}</a
            >
            <template v-else>{{ seg.text }}</template>
        </template>
    </p>
</template>
