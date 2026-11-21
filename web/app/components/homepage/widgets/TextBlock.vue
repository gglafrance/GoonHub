<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// text_block widget: free text (mini markdown) and shortcut links from the section config.
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData;
}>();

interface TextBlockLink {
    label: string;
    url: string;
}

const html = computed(() => renderMiniMarkdown(String(props.section.config?.markdown ?? '')));

const links = computed<TextBlockLink[]>(() => {
    const raw = props.section.config?.links;
    if (!Array.isArray(raw)) return [];
    return raw.filter(
        (l): l is TextBlockLink =>
            !!l && typeof l.label === 'string' && typeof l.url === 'string' && isSafeLinkUrl(l.url),
    );
});
</script>

<template>
    <div class="border-border space-y-3 rounded-lg border bg-white/2 p-4">
        <!-- eslint-disable-next-line vue/no-v-html -->
        <div v-if="html" class="text-muted space-y-2 text-xs leading-relaxed" v-html="html" />
        <div v-if="links.length" class="flex flex-wrap gap-2">
            <NuxtLink
                v-for="link in links"
                :key="link.url + link.label"
                :to="link.url"
                :target="link.url.startsWith('http') ? '_blank' : undefined"
                class="border-border hover:border-lava/40 hover:text-lava text-dim flex items-center
                    gap-1 rounded-md border px-2.5 py-1 text-[11px] transition-all"
            >
                <Icon name="heroicons:arrow-top-right-on-square" size="12" />
                {{ link.label }}
            </NuxtLink>
        </div>
        <p v-if="!html && !links.length" class="text-dim text-xs">Empty text block</p>
    </div>
</template>
