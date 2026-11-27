<script setup lang="ts">
import type { ProfileEdit, ProfileTabConfig } from '~/types/profile';
import type { Audience } from '~/types/social';

// Tabs and layout editor (orchestrator): the tabs (order, shown, titles, icons, custom tabs
// added or deleted) and the sections of the selected tab. Everything is saved with the button
// (PUT /profiles/me/tabs); the audiences of the blocks are privacy settings, saved at once.
const props = defineProps<{ state: ProfileEdit }>();
const emit = defineEmits<{ saved: [] }>();

const api = useApiProfiles();
const socialApi = useApiSocial();
const tabs = ref<ProfileTabConfig[]>([]);
const privacy = ref(props.state.privacy);
const selected = ref('profile');
const saving = ref(false);
const dirty = ref(false);
const error = ref('');

function sync() {
    tabs.value = JSON.parse(JSON.stringify(props.state.tabs)) as ProfileTabConfig[];
    privacy.value = props.state.privacy;
    if (!tabs.value.some((t) => t.id === selected.value)) selected.value = 'profile';
    dirty.value = false;
}
watch(() => props.state, sync);
onMounted(sync);

const current = computed(() => tabs.value.find((t) => t.id === selected.value));

function setTabs(next: ProfileTabConfig[]) {
    tabs.value = next;
    dirty.value = true;
}

function setSections(sections: ProfileTabConfig['sections']) {
    tabs.value = tabs.value.map((t) => (t.id === selected.value ? { ...t, sections } : t));
    dirty.value = true;
}

async function changeAudience(item: string, audience: Audience | 'default') {
    error.value = '';
    try {
        privacy.value = await socialApi.updatePrivacySettings({ items: { [item]: audience } });
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to change who can see it';
    }
}

async function persist(action: () => Promise<{ tabs: ProfileTabConfig[] }>) {
    saving.value = true;
    error.value = '';
    try {
        tabs.value = (await action()).tabs;
        dirty.value = false;
        emit('saved');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save the tabs';
    } finally {
        saving.value = false;
    }
}

const LIST_TAB_TEXT: Record<string, string> = {
    likes: 'This tab lists every video you liked (privacy: Liked videos).',
    ratings: 'This tab shows your ratings chart and the videos you rated (privacy: Ratings).',
    jizz: 'This tab lists the videos you came to (privacy: Jizz).',
};
</script>

<template>
    <div class="space-y-4">
        <ProfileEditTabList
            v-model:selected="selected"
            :tabs="tabs"
            :max-tabs="props.state.limits.max_tabs"
            @update="setTabs"
        />
        <ProfileEditSections
            v-if="current && tabHasSections(current.kind)"
            :key="current.id"
            :sections="current.sections ?? []"
            :state="props.state"
            :privacy="privacy"
            :all-ids="layoutIds(tabs)"
            @update="setSections"
            @audience="changeAudience"
        />
        <p v-else-if="current" class="text-dim border-border rounded-lg border px-3 py-4 text-xs">
            {{ LIST_TAB_TEXT[current.kind] }}
        </p>
        <div class="flex flex-wrap items-center gap-3">
            <button
                class="bg-lava hover:bg-lava/90 rounded-md px-3 py-1.5 text-xs font-medium
                    text-white disabled:opacity-50"
                :disabled="!dirty || saving"
                @click="persist(() => api.setTabs(tabs))"
            >
                {{ saving ? 'Saving...' : 'Save tabs and layout' }}
            </button>
            <button
                class="text-dim text-[11px] hover:text-white"
                :disabled="saving"
                @click="persist(api.resetTabs)"
            >
                Reset to default
            </button>
            <span v-if="dirty && !saving" class="text-dim text-[10px]">Unsaved changes</span>
            <span v-if="error" class="text-lava text-[11px]">{{ error }}</span>
        </div>
    </div>
</template>
