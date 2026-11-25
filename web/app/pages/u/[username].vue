<script setup lang="ts">
import type { Relationship, SocialListKind, SocialProfile } from '~/types/social';

// User page (minimal profile): identity, relation, follow/friend actions, relation lists.
// Thin orchestrator: the full profile (favorites, activity, stats...) adds blocks below.
const route = useRoute();
const api = useApiSocial();
const socialStore = useSocialStore();

definePageMeta({
    middleware: ['auth'],
});

const username = computed(() => String(route.params.username ?? ''));
const profile = ref<SocialProfile | null>(null);
const loading = ref(true);
const error = ref('');
const notFound = ref(false);
const activeList = ref<SocialListKind>('friends');

useHead({ title: () => username.value || 'Profile' });

async function load() {
    loading.value = true;
    error.value = '';
    notFound.value = false;
    try {
        profile.value = await api.getSocialProfile(username.value);
        if (!profile.value.visibility.friends_list && profile.value.visibility.followers_list) {
            activeList.value = 'followers';
        }
    } catch (e: unknown) {
        profile.value = null;
        notFound.value = (e as { status?: number }).status === 404;
        error.value = e instanceof Error ? e.message : 'Failed to load the profile';
    } finally {
        loading.value = false;
    }
}

// A follow or a friendship changes the counts and possibly what is visible
async function onRelationshipUpdate(rel: Relationship) {
    if (profile.value) profile.value.relationship = rel;
    await load();
}

function openList(kind: SocialListKind) {
    activeList.value = kind;
}

watch(username, load);
watch(
    () => socialStore.version,
    () => {
        if (profile.value && !profile.value.relationship.self) load();
    },
);
onMounted(load);
</script>

<template>
    <div class="mx-auto max-w-3xl space-y-4 px-4 py-6 sm:px-5">
        <div v-if="loading && !profile" class="flex justify-center py-12">
            <LoadingSpinner />
        </div>
        <div v-else-if="notFound" class="glass-panel text-dim p-8 text-center text-sm">
            <Icon name="heroicons:user" size="24" class="mb-2" />
            <p>No user named “{{ username }}”.</p>
        </div>
        <div
            v-else-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>
        <template v-else-if="profile">
            <SocialProfileHeader
                :profile="profile"
                @update="onRelationshipUpdate"
                @open="openList"
            />
            <template v-if="profile.visibility.profile">
                <SocialProfileLists v-model:active="activeList" :profile="profile" />
            </template>
            <div v-else class="glass-panel text-dim p-8 text-center text-xs">
                <Icon name="heroicons:lock-closed" size="20" class="mb-2" />
                <p>{{ profile.user.username }} keeps their profile private.</p>
            </div>
        </template>
    </div>
</template>
