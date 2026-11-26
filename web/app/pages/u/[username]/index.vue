<script setup lang="ts">
import type { Profile } from '~/types/profile';

// Profile tab of a profile page: the blocks the viewer may see (the parent page loads the
// profile and passes it).
const props = defineProps<{ profile: Profile; owner: boolean }>();

definePageMeta({
    middleware: ['auth'],
});

// Other viewers do not see the empty blocks; a locked placeholder stays
const blocks = computed(() =>
    (props.profile.blocks ?? []).filter((b) => props.owner || b.locked || !b.empty),
);
</script>

<template>
    <div class="space-y-4">
        <ProfileBlockRenderer
            v-for="block in blocks"
            :key="block.type"
            :block="block"
            :owner="props.owner"
        />
        <div v-if="blocks.length === 0" class="glass-panel text-dim p-6 text-center text-xs">
            {{
                props.owner
                    ? 'Your profile shows no block yet: Edit profile > Blocks.'
                    : 'Nothing to show here.'
            }}
        </div>
    </div>
</template>
