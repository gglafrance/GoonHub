<script setup lang="ts">
defineProps<{
    sceneId: string;
}>();
</script>

<template>
    <div class="space-y-1">
        <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">PornDB Scene</h3>
        <a
            :href="`https://theporndb.net/scenes/${sceneId}`"
            target="_blank"
            rel="noopener noreferrer"
            class="text-lava hover:text-lava-glow -mx-2 inline-flex items-center gap-1.5 rounded-md
                px-2 py-1 text-sm transition-colors hover:bg-white/3"
        >
            View on ThePornDB
            <Icon name="heroicons:arrow-top-right-on-square" size="12" />
        </a>
    </div>
</template>
