<script setup lang="ts">
import type { DuplicationConfig } from '~/types/duplicates';

const { getConfig, updateConfig } = useApiDuplicates();
const { message, error, clearMessages } = useSettingsMessage();

const loading = ref(true);
const saving = ref(false);
const config = ref<DuplicationConfig | null>(null);

const loadConfig = async () => {
    loading.value = true;
    clearMessages();
    try {
        config.value = await getConfig();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load duplication config';
    } finally {
        loading.value = false;
    }
};

const handleSave = async (newConfig: DuplicationConfig) => {
    saving.value = true;
    clearMessages();
    try {
        await updateConfig(newConfig as unknown as Record<string, unknown>);
        config.value = newConfig;
        message.value = 'Duplication config saved successfully';
        setTimeout(() => {
            message.value = '';
        }, 3000);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save duplication config';
    } finally {
        saving.value = false;
    }
};

onMounted(loadConfig);
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

        <div class="glass-panel p-5">
            <div class="mb-4">
                <h3 class="text-sm font-semibold text-white">Duplication Detection</h3>
                <p class="text-dim mt-0.5 text-[11px]">
                    Configure fingerprint mode and detection thresholds for duplicate scene matching.
                </p>
            </div>

            <div v-if="loading" class="text-dim py-8 text-center text-xs">Loading...</div>

            <DuplicatesConfigPanel
                v-else-if="config"
                :config="config"
                @save="handleSave"
            />
        </div>
    </div>
</template>
