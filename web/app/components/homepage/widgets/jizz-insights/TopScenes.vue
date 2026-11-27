<script setup lang="ts">
// Top scenes by dated O of the jizz_insights widget.
defineProps<{
    scenes: { id: number; title: string; thumbnail_path: string; count: number }[];
}>();

const number = new Intl.NumberFormat();
</script>

<template>
    <figure>
        <figcaption class="text-dim mb-2 text-[10px] font-medium uppercase">Top scenes</figcaption>
        <p v-if="scenes.length === 0" class="text-dim text-xs">No scene left in the library</p>
        <ol v-else class="space-y-1">
            <li v-for="(scene, index) in scenes" :key="scene.id">
                <NuxtLink
                    :to="`/watch/${scene.id}`"
                    class="group flex items-center gap-2.5 rounded-md px-1 py-1 transition-colors
                        hover:bg-white/5"
                >
                    <span
                        class="w-4 shrink-0 text-right font-mono text-xs font-bold"
                        :class="index === 0 ? 'text-lava' : 'text-dim'"
                    >
                        {{ index + 1 }}
                    </span>
                    <img
                        v-if="scene.thumbnail_path"
                        :src="`/thumbnails/${scene.id}?size=sm`"
                        :alt="scene.title"
                        loading="lazy"
                        class="h-8 w-14 shrink-0 rounded bg-black/50 object-cover"
                    />
                    <div v-else class="h-8 w-14 shrink-0 rounded bg-black/50" aria-hidden="true" />
                    <span
                        class="min-w-0 flex-1 truncate text-xs text-white/90 group-hover:text-white"
                        :title="scene.title"
                    >
                        {{ scene.title || `Scene ${scene.id}` }}
                    </span>
                    <span class="shrink-0 font-mono text-[11px] text-white tabular-nums">
                        {{ number.format(scene.count) }} O
                    </span>
                </NuxtLink>
            </li>
        </ol>
    </figure>
</template>
