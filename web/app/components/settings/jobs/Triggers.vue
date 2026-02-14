<script setup lang="ts">
import type { TriggerConfig } from '~/types/jobs';

const { fetchTriggerConfig, updateTriggerConfig } = useApi();

const loading = ref(true);
const error = ref('');
const message = ref('');
const configs = ref<TriggerConfig[]>([]);
const editingPhase = ref<string | null>(null);

const editForm = ref({
    trigger_type: '' as string,
    after_phase: null as string | null,
    cron_expression: null as string | null,
});

const saving = ref(false);

const cronPresets = [
    { label: 'Every 5 min', value: '*/5 * * * *' },
    { label: 'Every 15 min', value: '*/15 * * * *' },
    { label: 'Every 30 min', value: '*/30 * * * *' },
    { label: 'Hourly', value: '0 * * * *' },
    { label: 'Every 6h', value: '0 */6 * * *' },
    { label: 'Daily', value: '0 0 * * *' },
];

const showAdvancedCron = ref(false);

const loadConfigs = async () => {
    loading.value = true;
    error.value = '';
    try {
        configs.value = await fetchTriggerConfig();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load trigger config';
    } finally {
        loading.value = false;
    }
};

const startEdit = (phase: string) => {
    const cfg = configs.value.find((c) => c.phase === phase);
    if (!cfg) return;
    editingPhase.value = phase;
    editForm.value = {
        trigger_type: cfg.trigger_type,
        after_phase: cfg.after_phase,
        cron_expression: cfg.cron_expression,
    };
    showAdvancedCron.value = !cronPresets.some((p) => p.value === cfg.cron_expression);
};

const cancelEdit = () => {
    editingPhase.value = null;
    showAdvancedCron.value = false;
};

const saveConfig = async (phase: string) => {
    saving.value = true;
    error.value = '';
    message.value = '';
    try {
        configs.value = await updateTriggerConfig({
            phase,
            trigger_type: editForm.value.trigger_type,
            after_phase:
                editForm.value.trigger_type === 'after_job' ? editForm.value.after_phase : null,
            cron_expression:
                editForm.value.trigger_type === 'scheduled' ? editForm.value.cron_expression : null,
        });
        editingPhase.value = null;
        showAdvancedCron.value = false;
        message.value = `Trigger for ${phaseLabel(phase)} updated`;
        setTimeout(() => {
            message.value = '';
        }, 3000);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to update trigger config';
    } finally {
        saving.value = false;
    }
};

const phaseLabel = (phase: string): string => {
    switch (phase) {
        case 'metadata':
            return 'Metadata';
        case 'thumbnail':
            return 'Thumbnail';
        case 'sprites':
            return 'Sprites';
        case 'animated_thumbnails':
            return 'Previews & Clips';
        case 'fingerprint':
            return 'Fingerprint';
        case 'scan':
            return 'Library Scan';
        default:
            return phase;
    }
};

const phaseIcon = (phase: string): string => {
    switch (phase) {
        case 'metadata':
            return 'heroicons:document-text';
        case 'thumbnail':
            return 'heroicons:photo';
        case 'sprites':
            return 'heroicons:squares-2x2';
        case 'animated_thumbnails':
            return 'heroicons:play-circle';
        case 'fingerprint':
            return 'heroicons:finger-print';
        case 'scan':
            return 'heroicons:folder-open';
        default:
            return 'heroicons:cog-6-tooth';
    }
};

const phaseDescription = (phase: string): string => {
    switch (phase) {
        case 'metadata':
            return 'Extract duration, resolution, codec info';
        case 'thumbnail':
            return 'Scene previews and static marker thumbnails';
        case 'sprites':
            return 'Build sprite sheets and VTT files';
        case 'animated_thumbnails':
            return 'Hover preview videos and animated marker clips';
        case 'fingerprint':
            return 'Audio/visual fingerprint for duplication detection';
        case 'scan':
            return 'Discover new videos in storage paths';
        default:
            return '';
    }
};

const triggerBadgeClass = (type: string): string => {
    switch (type) {
        case 'on_import':
            return 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30';
        case 'after_job':
            return 'bg-blue-500/15 text-blue-400 border-blue-500/30';
        case 'manual':
            return 'bg-white/5 text-dim border-white/10';
        case 'scheduled':
            return 'bg-amber-500/15 text-amber-400 border-amber-500/30';
        default:
            return 'bg-white/5 text-dim border-white/10';
    }
};

const triggerLabel = (type: string): string => {
    switch (type) {
        case 'on_import':
            return 'On Import';
        case 'after_job':
            return 'After Job';
        case 'manual':
            return 'Manual';
        case 'scheduled':
            return 'Scheduled';
        default:
            return type;
    }
};

const availableAfterPhases = (currentPhase: string) => {
    return ['metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'fingerprint'].filter(
        (p) => p !== currentPhase,
    );
};

const triggerTypes = (phase: string) => {
    // Scan only supports manual and scheduled
    if (phase === 'scan') {
        return [
            {
                value: 'manual',
                label: 'Manual',
                description: 'Triggered manually via Jobs > Manual',
            },
            {
                value: 'scheduled',
                label: 'Scheduled',
                description: 'Runs on a cron schedule',
            },
        ];
    }

    const types = [
        {
            value: 'after_job',
            label: 'After Job',
            description: 'Runs after another phase completes',
        },
        { value: 'manual', label: 'Manual', description: 'Triggered manually via API' },
        {
            value: 'scheduled',
            label: 'Scheduled',
            description: 'Runs on a cron schedule',
        },
    ];
    if (phase === 'metadata') {
        types.unshift({
            value: 'on_import',
            label: 'On Import',
            description: 'Runs immediately when a scene is uploaded',
        });
    }
    return types;
};

onMounted(() => {
    loadConfigs();
});
</script>

<template>
    <div class="space-y-4">
        <div class="glass-panel p-5">
            <div class="mb-4">
                <h3 class="text-sm font-semibold text-white">Phase Triggers</h3>
                <p class="text-dim mt-1 text-[11px]">
                    Configure when each processing phase runs. Changes affect future uploads and
                    scheduled processing.
                </p>
            </div>

            <div
                v-if="error"
                class="border-lava/20 bg-lava/5 text-lava mb-4 rounded-lg border px-3 py-2 text-xs"
            >
                {{ error }}
            </div>

            <div
                v-if="message"
                class="mb-4 rounded-lg border border-emerald-500/20 bg-emerald-500/5 px-3 py-2
                    text-xs text-emerald-400"
            >
                {{ message }}
            </div>

            <div v-if="loading" class="text-dim py-8 text-center text-xs">Loading...</div>

            <div v-else class="space-y-3">
                <div
                    v-for="cfg in configs"
                    :key="cfg.phase"
                    class="rounded-lg border border-white/5 bg-white/2 p-4"
                >
                    <!-- Display mode -->
                    <div v-if="editingPhase !== cfg.phase">
                        <div class="flex items-center justify-between">
                            <div class="flex items-center gap-3">
                                <div
                                    class="flex h-7 w-7 items-center justify-center rounded-md
                                        border border-white/10 bg-white/5"
                                >
                                    <Icon :name="phaseIcon(cfg.phase)" size="14" class="text-dim" />
                                </div>
                                <div>
                                    <div class="text-xs font-medium text-white">
                                        {{ phaseLabel(cfg.phase) }}
                                    </div>
                                    <div class="text-dim text-[10px]">
                                        {{ phaseDescription(cfg.phase) }}
                                    </div>
                                </div>
                            </div>
                            <div class="flex items-center gap-2">
                                <span
                                    class="inline-block rounded-full border px-2 py-0.5 text-[10px]
                                        font-medium"
                                    :class="triggerBadgeClass(cfg.trigger_type)"
                                >
                                    {{ triggerLabel(cfg.trigger_type) }}
                                </span>
                                <span
                                    v-if="cfg.trigger_type === 'after_job' && cfg.after_phase"
                                    class="text-dim text-[10px]"
                                >
                                    (after {{ phaseLabel(cfg.after_phase) }})
                                </span>
                                <span
                                    v-if="cfg.trigger_type === 'scheduled' && cfg.cron_expression"
                                    class="text-dim font-mono text-[10px]"
                                >
                                    {{ cfg.cron_expression }}
                                </span>
                                <button
                                    class="text-dim ml-2 text-[11px] transition-colors
                                        hover:text-white"
                                    @click="startEdit(cfg.phase)"
                                >
                                    Edit
                                </button>
                            </div>
                        </div>
                    </div>

                    <!-- Edit mode -->
                    <div v-else class="space-y-3">
                        <div class="flex items-center justify-between">
                            <div class="flex items-center gap-2 text-xs font-medium text-white">
                                <Icon :name="phaseIcon(cfg.phase)" size="13" class="text-dim" />
                                {{ phaseLabel(cfg.phase) }}
                            </div>
                            <button
                                class="text-dim text-[11px] transition-colors hover:text-white"
                                @click="cancelEdit"
                            >
                                Cancel
                            </button>
                        </div>

                        <!-- Trigger type selector -->
                        <div class="flex flex-wrap gap-1.5">
                            <button
                                v-for="tt in triggerTypes(cfg.phase)"
                                :key="tt.value"
                                :class="[
                                    `rounded-full border px-2.5 py-1 text-[11px] font-medium
                                    transition-colors`,
                                    editForm.trigger_type === tt.value
                                        ? 'border-white/20 bg-white/10 text-white'
                                        : 'text-dim border-white/5 bg-white/2 hover:text-white',
                                ]"
                                :title="tt.description"
                                @click="editForm.trigger_type = tt.value"
                            >
                                {{ tt.label }}
                            </button>
                        </div>

                        <!-- After phase selector -->
                        <div
                            v-if="editForm.trigger_type === 'after_job'"
                            class="flex items-center gap-2"
                        >
                            <span class="text-dim text-[11px]">Run after:</span>
                            <select
                                v-model="editForm.after_phase"
                                class="border-border bg-surface rounded-lg border px-2 py-1
                                    text-[11px] text-white focus:border-white/20 focus:outline-none"
                            >
                                <option
                                    v-for="phase in availableAfterPhases(cfg.phase)"
                                    :key="phase"
                                    :value="phase"
                                >
                                    {{ phaseLabel(phase) }}
                                </option>
                            </select>
                        </div>

                        <!-- Cron expression -->
                        <div v-if="editForm.trigger_type === 'scheduled'" class="space-y-2">
                            <div v-if="!showAdvancedCron" class="flex flex-wrap gap-1.5">
                                <button
                                    v-for="preset in cronPresets"
                                    :key="preset.value"
                                    :class="[
                                        `rounded-full border px-2 py-0.5 text-[10px] font-medium
                                        transition-colors`,
                                        editForm.cron_expression === preset.value
                                            ? 'border-amber-500/30 bg-amber-500/15 text-amber-400'
                                            : 'text-dim border-white/5 bg-white/2 hover:text-white',
                                    ]"
                                    @click="editForm.cron_expression = preset.value"
                                >
                                    {{ preset.label }}
                                </button>
                                <button
                                    class="text-dim rounded-full border border-white/5 bg-white/2
                                        px-2 py-0.5 text-[10px] font-medium transition-colors
                                        hover:text-white"
                                    @click="showAdvancedCron = true"
                                >
                                    Advanced
                                </button>
                            </div>
                            <div v-else class="flex items-center gap-2">
                                <input
                                    v-model="editForm.cron_expression"
                                    type="text"
                                    placeholder="*/5 * * * *"
                                    class="border-border bg-surface w-40 rounded-lg border px-2 py-1
                                        font-mono text-[11px] text-white focus:border-white/20
                                        focus:outline-none"
                                />
                                <button
                                    class="text-dim text-[10px] transition-colors hover:text-white"
                                    @click="showAdvancedCron = false"
                                >
                                    Presets
                                </button>
                            </div>
                            <div class="text-dim text-[10px]">
                                Format: minute hour day month weekday
                            </div>
                        </div>

                        <!-- Save button -->
                        <div class="flex justify-end pt-1">
                            <button
                                :disabled="saving"
                                class="bg-lava hover:bg-lava/90 rounded-lg px-3 py-1 text-[11px]
                                    font-medium text-white transition-colors disabled:opacity-50"
                                @click="saveConfig(cfg.phase)"
                            >
                                {{ saving ? 'Saving...' : 'Save' }}
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>
