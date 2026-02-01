<script setup lang="ts">
import type { StoragePathWithCount } from '~/types/explorer';

const props = defineProps<{
    storagePath: StoragePathWithCount;
}>();

const router = useRouter();

const handleClick = () => {
    router.push(`/explorer/${props.storagePath.id}`);
};
</script>

<template>
    <button
        @click="handleClick"
        class="border-border bg-panel hover:border-lava/30 group flex flex-col items-start
            gap-3 rounded-xl border p-4 text-left transition-all"
    >
        <div class="flex w-full items-start justify-between">
            <div
                class="bg-lava/10 group-hover:bg-lava/20 flex h-10 w-10 items-center justify-center
                    rounded-lg transition-colors"
            >
                <Icon name="heroicons:folder" size="20" class="text-lava" />
            </div>
            <span
                v-if="storagePath.is_default"
                class="bg-emerald/10 text-emerald rounded-full px-2 py-0.5 text-[10px] font-medium"
            >
                Default
            </span>
        </div>

        <div class="w-full min-w-0">
            <h3 class="truncate font-medium text-white group-hover:text-white">
                {{ storagePath.name }}
            </h3>
            <p class="text-dim mt-0.5 truncate text-xs">
                {{ storagePath.path }}
            </p>
        </div>

        <div class="text-dim flex items-center gap-1.5 text-xs">
            <Icon name="heroicons:film" size="14" />
            <span>{{ storagePath.scene_count }} scenes</span>
        </div>
    </button>
</template>
