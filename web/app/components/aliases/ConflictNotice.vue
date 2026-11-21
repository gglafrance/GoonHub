<script setup lang="ts">
import type { EntityConflict } from '~/types/conflict';

// Tells that a name or alias is already used by another actor/studio, with a link to it.
const props = defineProps<{
    conflict: EntityConflict<{ uuid: string; name: string }>;
    kind: 'actor' | 'studio';
}>();

const link = computed(() => `/${props.kind}s/${props.conflict.existing.uuid}`);
const what = computed(() => (props.conflict.field === 'name' ? 'This name' : 'The alias'));
</script>

<template>
    <div
        class="border-lava/20 bg-lava/5 text-lava flex flex-wrap items-center gap-x-1 rounded-lg
            border px-3 py-2 text-xs"
    >
        <Icon name="heroicons:exclamation-triangle" size="14" class="shrink-0" />
        <span>
            {{ what }} "{{ conflict.value }}" is already used by the {{ kind }}
            <NuxtLink :to="link" class="font-semibold underline hover:text-white">
                {{ conflict.existing.name }}</NuxtLink
            >.
        </span>
        <slot />
    </div>
</template>
