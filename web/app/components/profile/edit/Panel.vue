<script setup lang="ts">
import type { ProfileEdit } from '~/types/profile';

// Edit mode of the owner's profile (orchestrator): identity and images, tabs and their section
// layouts, custom lists, and the two showcases. Emits changed when the profile page should reload.
const emit = defineEmits<{ changed: [] }>();

const api = useApiProfiles();
const perms = useMyPermissions();
const state = ref<ProfileEdit | null>(null);
const error = ref('');
const tab = ref<'profile' | 'layout' | 'lists' | 'videos' | 'markers'>('profile');

const tabs = computed(() =>
    [
        { id: 'profile' as const, label: 'Profile', icon: 'heroicons:user-circle', show: true },
        {
            id: 'layout' as const,
            label: 'Tabs & layout',
            icon: 'heroicons:squares-2x2',
            show: true,
        },
        {
            id: 'lists' as const,
            label: 'Custom lists',
            icon: 'heroicons:queue-list',
            show: perms.has('scenes:view'),
        },
        {
            id: 'videos' as const,
            label: 'Favorite videos',
            icon: 'heroicons:star',
            show: perms.has('scenes:view'),
        },
        {
            id: 'markers' as const,
            label: 'Favorite markers',
            icon: 'heroicons:bookmark',
            show: perms.has('scenes:view'),
        },
    ].filter((t) => t.show),
);

async function load() {
    error.value = '';
    try {
        state.value = await api.getMyProfile();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load your profile';
    }
}

async function changed() {
    await load();
    emit('changed');
}

function onSaved(next: ProfileEdit) {
    state.value = next;
    emit('changed');
}

onMounted(load);
</script>

<template>
    <div class="glass-panel border-lava/20 p-4">
        <div class="mb-4 flex flex-wrap gap-1">
            <button
                v-for="t in tabs"
                :key="t.id"
                class="flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[11px] font-medium
                    transition-all"
                :class="
                    tab === t.id
                        ? 'bg-lava/10 text-lava'
                        : 'text-dim hover:bg-white/5 hover:text-white'
                "
                @click="tab = t.id"
            >
                <Icon :name="t.icon" size="13" />
                {{ t.label }}
            </button>
        </div>
        <p v-if="error" class="text-lava text-xs">{{ error }}</p>
        <div v-else-if="!state" class="flex justify-center py-6"><LoadingSpinner /></div>
        <template v-else>
            <div v-if="tab === 'profile'" class="grid gap-6 md:grid-cols-2">
                <ProfileEditIdentity :state="state" @saved="onSaved" />
                <ProfileEditImages :state="state" @changed="changed" />
            </div>
            <ProfileEditTabs v-else-if="tab === 'layout'" :state="state" @saved="changed" />
            <ProfileEditLists v-else-if="tab === 'lists'" :state="state" @saved="changed" />
            <ProfileEditShowcase
                v-else-if="tab === 'videos'"
                :state="state"
                kind="scenes"
                @saved="changed"
            />
            <ProfileEditShowcase v-else :state="state" kind="markers" @saved="changed" />
        </template>
    </div>
</template>
