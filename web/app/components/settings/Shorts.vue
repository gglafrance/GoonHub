<script setup lang="ts">
import type { ShortsSearchFilter, ShortsSettings } from '~/types/shorts';
import { DEFAULT_SHORTS_SEARCH, SHORTS_SEARCH_OPTIONS } from '~/types/shorts';
import { formatShortSpan } from '~/composables/useShortEditor';

const { getShortsSettings, updateShortsSettings } = useApiShorts();
const { message, error, clearMessages } = useSettingsMessage();

const loading = ref(true);
const saving = ref(false);
const settings = ref<ShortsSettings | null>(null);
const maxDuration = ref(60);
const saveDir = ref('');
const searchDefault = ref<ShortsSearchFilter>(DEFAULT_SHORTS_SEARCH);
const saveDirError = ref('');
const limitError = ref('');

// Quick picks next to the free seconds field
const limitPresets = [60, 120];

const limitValid = computed(
    () =>
        Number.isInteger(maxDuration.value) &&
        maxDuration.value >= 1 &&
        maxDuration.value <= 2147483647,
);

const apply = (s: ShortsSettings) => {
    settings.value = s;
    maxDuration.value = s.max_duration;
    searchDefault.value = s.search_default ?? DEFAULT_SHORTS_SEARCH;
    saveDir.value = s.save_dir ?? s.effective_save_dir ?? s.default_save_dir;
};

onMounted(async () => {
    try {
        apply(await getShortsSettings());
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load shorts settings';
    } finally {
        loading.value = false;
    }
});

// The default folder is stored as null so it keeps following the default
// storage path.
const usesDefault = computed(
    () => !saveDir.value.trim() || saveDir.value.trim() === settings.value?.default_save_dir,
);

const resetToDefault = () => {
    saveDir.value = settings.value?.default_save_dir ?? '';
    saveDirError.value = '';
};

const save = async () => {
    clearMessages();
    saveDirError.value = '';
    limitError.value = '';
    if (!limitValid.value) {
        limitError.value = 'Enter a whole number of seconds, 1 or more';
        return;
    }
    saving.value = true;
    try {
        apply(
            await updateShortsSettings({
                max_duration: maxDuration.value,
                save_dir: usesDefault.value ? null : saveDir.value.trim(),
                search_default: searchDefault.value,
            }),
        );
        message.value = 'Shorts settings saved';
    } catch (e: unknown) {
        saveDirError.value = e instanceof Error ? e.message : 'Failed to save shorts settings';
    } finally {
        saving.value = false;
    }
};
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

        <div v-if="loading" class="text-dim py-8 text-center text-xs">Loading...</div>

        <div v-else-if="settings" class="glass-panel space-y-6 p-5">
            <div>
                <h3 class="text-sm font-semibold text-white">Shorts</h3>
                <p class="text-dim mt-0.5 text-[11px]">
                    The Shorts feed and where shorts cut from a scene are saved.
                </p>
            </div>

            <div class="space-y-1.5">
                <label class="text-xs font-medium text-white">Length limit</label>
                <div class="flex flex-wrap items-center gap-2">
                    <div class="relative">
                        <input
                            v-model.number="maxDuration"
                            type="number"
                            min="1"
                            step="1"
                            inputmode="numeric"
                            class="border-border bg-surface block w-32 rounded-lg border py-2 pr-14
                                pl-3 text-xs text-white focus:border-white/20 focus:outline-none"
                            :class="{ 'border-lava/50': limitError }"
                            @input="limitError = ''"
                        />
                        <span
                            class="text-dim pointer-events-none absolute top-1/2 right-3
                                -translate-y-1/2 text-[11px]"
                        >
                            seconds
                        </span>
                    </div>
                    <button
                        v-for="p in limitPresets"
                        :key="p"
                        type="button"
                        class="rounded-full border px-3 py-1 text-[11px] transition-colors"
                        :class="
                            maxDuration === p
                                ? 'border-lava/40 bg-lava/10 text-white'
                                : 'border-border text-dim hover:border-white/20 hover:text-white'
                        "
                        @click="((maxDuration = p), (limitError = ''))"
                    >
                        {{ p }}s
                    </button>
                    <span
                        v-if="limitValid && maxDuration >= 60"
                        class="text-dim font-mono text-[11px]"
                    >
                        = {{ formatShortSpan(maxDuration) }}
                    </span>
                </div>
                <p v-if="limitError" class="text-lava text-[11px]">{{ limitError }}</p>
                <p class="text-dim text-[11px]">
                    Scenes this long or shorter appear in the Shorts feed. Created shorts longer
                    than this are kept but hidden from the feed.
                </p>
            </div>

            <div class="space-y-1.5">
                <label class="text-xs font-medium text-white">Search page filter</label>
                <select
                    v-model="searchDefault"
                    class="border-border bg-surface block w-56 rounded-lg border px-3 py-2 text-xs
                        text-white focus:border-white/20 focus:outline-none"
                >
                    <option v-for="o in SHORTS_SEARCH_OPTIONS" :key="o.value" :value="o.value">
                        {{ o.label }}
                    </option>
                </select>
                <p class="text-dim text-[11px]">
                    What the Shorts filter on the search page starts on, for everyone. Anyone can
                    still change it while searching.
                </p>
            </div>

            <SettingsShortsSaveLocation
                v-model="saveDir"
                :settings="settings"
                :uses-default="usesDefault"
                :error="saveDirError"
                @reset="resetToDefault"
            />

            <div class="flex justify-end">
                <button
                    :disabled="saving"
                    class="bg-lava hover:bg-lava-glow flex items-center gap-1.5 rounded-lg px-4
                        py-1.5 text-xs font-semibold text-white transition-all
                        disabled:cursor-not-allowed disabled:opacity-40"
                    @click="save"
                >
                    <Icon v-if="saving" name="svg-spinners:ring-resize" size="12" />
                    Save
                </button>
            </div>
        </div>
    </div>
</template>
