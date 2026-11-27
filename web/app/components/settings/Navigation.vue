<script setup lang="ts">
import type { NavLayout } from '~/types/navLayout';

// Settings > Navigation: the per-account layout of the two header bars (docs/NAVIGATION.md).
// The draft is edited with drag and drop or the toolbar of the selected entry, previewed live,
// and stored on Save. Only the items the user has the permission for are offered.
const perms = useMyPermissions();
const store = useNavLayoutStore();
const { message, error, clearMessages } = useSettingsMessage();

const loading = ref(true);
const saving = ref(false);
const baseline = ref<NavLayout>(completeNavLayout(null, perms.has));
const draft = ref<NavLayout>(cloneNavLayout(baseline.value));
// Reset to default was clicked: saving deletes the stored layout (later defaults then apply)
const pendingReset = ref(false);
const editor = provideNavLayoutEditor(draft);

const dirty = computed(() => pendingReset.value || !sameNavLayout(draft.value, baseline.value));

function syncFromStore() {
    baseline.value = completeNavLayout(store.layout, perms.has);
    draft.value = cloneNavLayout(baseline.value);
    pendingReset.value = false;
}

async function load() {
    loading.value = true;
    await store.refresh();
    syncFromStore();
    loading.value = false;
}

// Changed from another tab or device: follow it unless there are unsaved changes
watch(
    () => store.layout,
    () => {
        if (!dirty.value && !saving.value) syncFromStore();
    },
);

async function save() {
    clearMessages();
    saving.value = true;
    try {
        const isDefault = sameNavLayout(draft.value, completeNavLayout(null, perms.has));
        if (pendingReset.value && isDefault) await store.reset();
        else await store.save(draft.value);
        syncFromStore();
        message.value = 'Navigation saved';
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save the navigation';
    } finally {
        saving.value = false;
    }
}

function discard() {
    clearMessages();
    draft.value = cloneNavLayout(baseline.value);
    pendingReset.value = false;
    editor.select(null);
}

function resetToDefault() {
    draft.value = completeNavLayout(null, perms.has);
    pendingReset.value = store.customized;
    editor.select(null);
}

function newFolder() {
    clearMessages();
    const id = nextNavFolderId(draft.value);
    const next = createNavFolder(draft.value, 'New folder', 'heroicons:folder', {
        bar: 'nav',
        zone: 'right',
    });
    if (next === draft.value) {
        error.value = `At most ${MAX_NAV_FOLDERS} folders`;
        return;
    }
    draft.value = next;
    editor.select(NAV_FOLDER_REF_PREFIX + id);
}

onMounted(load);
</script>

<template>
    <div class="space-y-6">
        <div
            v-if="message"
            class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2 text-xs"
        >
            {{ message }}
        </div>
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <div v-if="loading" class="flex justify-center py-8"><LoadingSpinner /></div>

        <template v-else>
            <SettingsNavigationPreview :layout="draft" />

            <div class="glass-panel space-y-5 p-5">
                <div class="flex flex-wrap items-start justify-between gap-2">
                    <div>
                        <h3 class="text-sm font-semibold text-white">Layout</h3>
                        <p class="text-dim mt-0.5 text-[11px]">
                            Drag items between the zones, into folders or to Hidden, or select one
                            and use the buttons below. Synced to all your devices.
                        </p>
                    </div>
                    <div class="flex items-center gap-3">
                        <button
                            type="button"
                            class="border-border text-dim hover:border-lava/30 flex items-center
                                gap-1.5 rounded-md border px-2.5 py-1 text-[11px] font-medium
                                transition-all hover:text-white"
                            @click="newFolder"
                        >
                            <Icon name="heroicons:folder-plus" size="14" />
                            New folder
                        </button>
                        <button
                            type="button"
                            class="text-dim hover:text-lava text-[11px] transition-colors"
                            @click="resetToDefault"
                        >
                            Reset to default
                        </button>
                    </div>
                </div>

                <SettingsNavigationBarEditor bar="top" />
                <SettingsNavigationBarEditor bar="nav" />
                <SettingsNavigationHiddenTray />
            </div>

            <SettingsNavigationToolbar />

            <SettingsNavigationMobileSummary :layout="draft" />

            <SettingsNavigationSaveBar
                v-if="dirty"
                :saving="saving"
                @save="save"
                @discard="discard"
            />
        </template>
    </div>
</template>
