<script setup lang="ts">
// Config form of the text_block widget. Widget config forms receive and emit the section config
// (v-model); the file name is the PascalCase section type.
const config = defineModel<Record<string, unknown>>({ required: true });

interface TextBlockLink {
    label: string;
    url: string;
}

const markdown = computed({
    get: () => String(config.value.markdown ?? ''),
    set: (v: string) => (config.value = { ...config.value, markdown: v }),
});

const links = computed<TextBlockLink[]>(() =>
    Array.isArray(config.value.links) ? (config.value.links as TextBlockLink[]) : [],
);

const setLinks = (next: TextBlockLink[]) => (config.value = { ...config.value, links: next });

const updateLink = (index: number, field: keyof TextBlockLink, value: string) =>
    setLinks(links.value.map((l, i) => (i === index ? { ...l, [field]: value } : l)));

const addLink = () => setLinks([...links.value, { label: '', url: '/' }]);
const removeLink = (index: number) => setLinks(links.value.filter((_, i) => i !== index));
</script>

<template>
    <div class="space-y-3">
        <div>
            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                Text
            </label>
            <textarea
                v-model="markdown"
                rows="5"
                maxlength="10000"
                placeholder="# Title, **bold**, *italic*, [link](/search?tags=Pup), - list"
                class="border-border bg-void/80 focus:border-lava/40 w-full rounded-lg border px-3
                    py-2 font-mono text-xs text-white focus:outline-none"
            />
        </div>
        <div>
            <div class="mb-1.5 flex items-center justify-between">
                <label class="text-dim text-[11px] font-medium tracking-wider uppercase">
                    Links
                </label>
                <button class="text-lava text-[11px] hover:underline" @click="addLink">
                    + Add link
                </button>
            </div>
            <div v-for="(link, index) in links" :key="index" class="mb-1.5 flex gap-1.5">
                <input
                    :value="link.label"
                    placeholder="Label"
                    class="border-border bg-void/80 w-1/3 rounded-md border px-2 py-1 text-xs
                        text-white"
                    @input="updateLink(index, 'label', ($event.target as HTMLInputElement).value)"
                />
                <input
                    :value="link.url"
                    placeholder="/search?tags=... or https://..."
                    class="border-border bg-void/80 flex-1 rounded-md border px-2 py-1 font-mono
                        text-xs text-white"
                    @input="updateLink(index, 'url', ($event.target as HTMLInputElement).value)"
                />
                <button class="text-dim hover:text-lava px-1" @click="removeLink(index)">
                    <Icon name="heroicons:x-mark" size="14" />
                </button>
            </div>
        </div>
    </div>
</template>
