<script setup lang="ts">
import type { Actor } from '~/types/actor';

const props = defineProps<{
    actor: Actor;
}>();

const imageUrl = computed(() => {
    return props.actor.image_url || null;
});

const genderInfo = computed(() => {
    const gender = props.actor.gender?.toLowerCase();
    if (!gender) return null;

    const genderMap: Record<string, { icon: string; label: string; color: string }> = {
        male: { icon: 'mdi:gender-male', label: 'Male', color: 'text-blue-400' },
        female: { icon: 'mdi:gender-female', label: 'Female', color: 'text-pink-400' },
        trans: {
            icon: 'mdi:gender-transgender',
            label: 'Trans',
            color: 'text-fuchsia-400',
        },
        'non-binary': {
            icon: 'mdi:gender-non-binary',
            label: 'Non-binary',
            color: 'text-purple-400',
        },
    };

    return genderMap[gender] || null;
});
</script>

<template>
    <NuxtLink
        :to="`/actors/${actor.uuid}`"
        class="group border-border bg-surface hover:border-border-hover hover:bg-elevated relative
            block overflow-hidden rounded-lg border transition-all duration-200"
    >
        <!-- Portrait aspect ratio (2:3) for actor images -->
        <div class="bg-void relative aspect-2/3 w-full">
            <img
                v-if="imageUrl"
                :src="imageUrl"
                class="absolute inset-0 h-full w-full object-cover transition-transform duration-300
                    group-hover:scale-[1.03]"
                :alt="actor.name"
                loading="lazy"
            />

            <div
                v-else
                class="text-dim group-hover:text-lava absolute inset-0 flex items-center
                    justify-center transition-colors"
            >
                <Icon name="heroicons:user" size="48" />
            </div>

            <!-- Gender badge -->
            <div
                v-if="genderInfo"
                :title="genderInfo.label"
                class="bg-void/90 absolute top-1.5 left-1.5 flex items-center justify-center rounded
                    p-1 backdrop-blur-sm"
            >
                <Icon :name="genderInfo.icon" :class="genderInfo.color" size="14" />
            </div>

            <!-- Video count badge -->
            <div
                v-if="actor.video_count !== undefined && actor.video_count > 0"
                class="bg-void/90 absolute right-1.5 bottom-1.5 rounded px-1.5 py-0.5 font-mono
                    text-[10px] font-medium text-white backdrop-blur-sm"
            >
                {{ actor.video_count }} {{ actor.video_count === 1 ? 'video' : 'videos' }}
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
                :title="actor.name"
            >
                {{ actor.name }}
            </h3>
            <div
                v-if="actor.nationality || actor.birthplace"
                class="text-dim mt-0.5 truncate text-xs"
            >
                {{ actor.nationality || actor.birthplace }}
            </div>
        </div>
    </NuxtLink>
</template>
