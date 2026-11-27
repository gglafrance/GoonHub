<script setup lang="ts">
import type { Profile } from '~/types/profile';

// A custom list of a profile, browsed in full (/u/<name>/lists/<id>, the "Browse" of its
// block): the list's own order, the user's card template, pages or infinite scroll.
const props = defineProps<{ profile: Profile; owner: boolean }>();

definePageMeta({
    middleware: ['auth'],
});

const route = useRoute();
const listId = computed(() => Number(route.params.id));
const title = ref('');

useHead({ title: () => `${title.value || 'List'} · ${displayNameOf(props.profile.user)}` });
</script>

<template>
    <section class="glass-panel space-y-3 p-4">
        <div class="flex items-center gap-2">
            <Icon name="heroicons:queue-list" size="14" class="text-lava" />
            <h2 class="text-xs font-semibold tracking-wide text-white uppercase">
                {{ title || 'List' }}
            </h2>
        </div>
        <ProfileSceneList
            v-if="Number.isInteger(listId) && listId > 0"
            :key="listId"
            :username="props.profile.user.username"
            :kind="`lists/${listId}`"
            :sort-options="[]"
            figure="none"
            empty-text="This list is empty, private, or does not exist."
            @loaded="title = $event.list?.title ?? ''"
        />
    </section>
</template>
