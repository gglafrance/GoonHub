<script setup lang="ts">
import type { StudioListItem } from '~/types/studio';

const props = defineProps<{
    studio: StudioListItem;
}>();

const logoUrl = computed(() => {
    return props.studio.logo || null;
});
</script>

<template>
    <NuxtLink
        :to="`/studios/${studio.uuid}`"
        class="group border-border bg-surface hover:border-border-hover hover:bg-elevated relative
            block overflow-hidden rounded-lg border transition-all duration-200"
    >
        <!-- Square aspect ratio for studio logos -->
        <div class="bg-void relative aspect-square w-full">
            <img
                v-if="logoUrl"
                :src="logoUrl"
                class="absolute inset-0 h-full w-full object-contain p-4 transition-transform
                    duration-300 group-hover:scale-[1.03]"
                :alt="studio.name"
                loading="lazy"
            />

            <div
                v-else
                class="text-dim group-hover:text-lava absolute inset-0 flex items-center
                    justify-center transition-colors"
            >
                <Icon name="heroicons:building-office-2" size="48" />
            </div>

            <!-- Scene count badge -->
            <div
                v-if="studio.scene_count !== undefined && studio.scene_count > 0"
                class="bg-void/90 absolute right-1.5 bottom-1.5 rounded px-1.5 py-0.5 font-mono
                    text-[10px] font-medium text-white backdrop-blur-sm"
            >
                {{ studio.scene_count }} {{ studio.scene_count === 1 ? 'scene' : 'scenes' }}
            </div>

            <!-- Hover overlay -->
            <div
                class="bg-lava/0 group-hover:bg-lava/5 absolute inset-0 transition-colors
                    duration-200"
            ></div>
        </div>

        <div class="p-2.5">
            <h3
                class="truncate text-[13px] font-medium text-white/90 transition-colors
                    group-hover:text-white"
                :title="studio.name"
            >
                {{ studio.name }}
            </h3>
            <p
                v-if="studio.short_name"
                class="text-dim truncate text-[11px]"
                :title="studio.short_name"
            >
                {{ studio.short_name }}
            </p>
        </div>
    </NuxtLink>
</template>
