<script setup lang="ts">
import type { Profile } from '~/types/profile';
import type { Relationship, SocialListKind } from '~/types/social';

// Profile page (orchestrator, docs/PROFILES.md): header, the blocks the viewer may see, the
// relation lists, and for the owner the edit mode (?edit=1).
const route = useRoute();
const router = useRouter();
const api = useApiProfiles();
const socialStore = useSocialStore();

definePageMeta({
    middleware: ['auth'],
});

const username = computed(() => String(route.params.username ?? ''));
const profile = ref<Profile | null>(null);
const loading = ref(true);
const error = ref('');
const notFound = ref(false);
const activeList = ref<SocialListKind>('friends');
const showLists = ref(false);
const listsEl = ref<HTMLElement | null>(null);

const owner = computed(() => !!profile.value?.relationship.self);
const editing = computed(() => owner.value && route.query.edit === '1');
// Other viewers do not see the empty blocks; a locked placeholder stays
const blocks = computed(() =>
    (profile.value?.blocks ?? []).filter((b) => owner.value || b.locked || !b.empty),
);

useHead({ title: () => (profile.value ? displayNameOf(profile.value.user) : username.value) });

async function load() {
    loading.value = true;
    error.value = '';
    notFound.value = false;
    try {
        profile.value = await api.getProfile(username.value);
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
    showLists.value = true;
    nextTick(() => listsEl.value?.scrollIntoView({ behavior: 'smooth', block: 'start' }));
}

function toggleEdit() {
    const query = { ...route.query };
    if (editing.value) delete query.edit;
    else query.edit = '1';
    router.replace({ query });
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
    <div class="mx-auto max-w-6xl space-y-4 px-4 py-6 sm:px-5">
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
            <ProfileHeader
                :profile="profile"
                :editing="editing"
                @update="onRelationshipUpdate"
                @open="openList"
                @edit="toggleEdit"
            />
            <ProfileEditPanel v-if="editing" @changed="load" />
            <template v-if="profile.visibility.profile">
                <ProfileBlockRenderer
                    v-for="block in blocks"
                    :key="block.type"
                    :block="block"
                    :owner="owner"
                />
                <div
                    v-if="blocks.length === 0"
                    class="glass-panel text-dim p-6 text-center text-xs"
                >
                    {{
                        owner
                            ? 'Your profile shows no block yet: Edit profile > Blocks.'
                            : 'Nothing to show here.'
                    }}
                </div>
                <div ref="listsEl">
                    <SocialProfileLists
                        v-if="showLists"
                        v-model:active="activeList"
                        :profile="profile"
                    />
                </div>
            </template>
            <div v-else class="glass-panel text-dim p-8 text-center text-xs">
                <Icon name="heroicons:lock-closed" size="20" class="mb-2" />
                <p>{{ displayNameOf(profile.user) }} keeps their profile private.</p>
            </div>
        </template>
    </div>
</template>
