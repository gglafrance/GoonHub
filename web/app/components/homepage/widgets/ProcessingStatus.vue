<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// processing_status widget (admins): job queues per phase and generated asset coverage.
interface PhaseStatus {
    phase: string;
    pending: number;
    running: number;
    failed: number;
}

interface Coverage {
    total: number;
    thumbnail: number;
    sprites: number;
    preview: number;
    fingerprint: number;
}

interface ProcessingPayload {
    forbidden?: boolean;
    phases?: PhaseStatus[];
    coverage?: Coverage;
    failed_window_hours?: number;
}

const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<ProcessingPayload>;
}>();

const emit = defineEmits<{ refresh: [] }>();

const REFRESH_INTERVAL_MS = 30_000;

const number = new Intl.NumberFormat();

const PHASE_LABELS: Record<string, string> = {
    metadata: 'Metadata',
    thumbnail: 'Thumbnails',
    sprites: 'Sprites',
    animated_thumbnails: 'Previews',
    fingerprint: 'Fingerprints',
};

const payload = computed(() => props.data.widget ?? null);
const phases = computed(() => payload.value?.phases ?? []);
const idle = computed(() => phases.value.every((p) => p.pending + p.running === 0));

const coverageItems = computed(() => {
    const c = payload.value?.coverage;
    if (!c) return [];
    const pct = (n: number) => (c.total > 0 ? (n / c.total) * 100 : 0);
    return [
        { key: 'thumbnail', label: 'Thumbnails', count: c.thumbnail, pct: pct(c.thumbnail) },
        { key: 'sprites', label: 'Sprites', count: c.sprites, pct: pct(c.sprites) },
        { key: 'preview', label: 'Previews', count: c.preview, pct: pct(c.preview) },
        {
            key: 'fingerprint',
            label: 'Fingerprints',
            count: c.fingerprint,
            pct: pct(c.fingerprint),
        },
    ];
});

// Auto refresh while the widget is visible on screen
const root = ref<HTMLElement | null>(null);
let timer: ReturnType<typeof setInterval> | null = null;
let observer: IntersectionObserver | null = null;
const visible = ref(false);

const startTimer = () => {
    if (timer) return;
    timer = setInterval(() => {
        if (visible.value && document.visibilityState === 'visible') emit('refresh');
    }, REFRESH_INTERVAL_MS);
};

onMounted(() => {
    if (root.value && 'IntersectionObserver' in window) {
        observer = new IntersectionObserver((entries) => {
            visible.value = entries.some((e) => e.isIntersecting);
        });
        observer.observe(root.value);
    } else {
        visible.value = true;
    }
    startTimer();
});

onBeforeUnmount(() => {
    if (timer) clearInterval(timer);
    observer?.disconnect();
});
</script>

<template>
    <div ref="root" class="border-border rounded-lg border bg-white/2 p-4">
        <p v-if="!payload || payload.forbidden" class="text-dim py-4 text-center text-xs">
            Processing status is only available to admins.
        </p>
        <template v-else>
            <div class="mb-3 flex items-center justify-between">
                <span
                    class="flex items-center gap-1.5 text-[11px]"
                    :class="idle ? 'text-emerald' : 'text-lava'"
                >
                    <span
                        class="h-1.5 w-1.5 rounded-full"
                        :class="idle ? 'bg-emerald' : 'bg-lava animate-pulse'"
                    />
                    {{ idle ? 'All queues idle' : 'Processing' }}
                </span>
                <NuxtLink
                    to="/settings?tab=jobs"
                    class="text-dim hover:text-lava flex items-center gap-1 text-[11px]
                        transition-colors"
                >
                    Jobs
                    <Icon name="heroicons:chevron-right" size="12" />
                </NuxtLink>
            </div>

            <div class="grid gap-4 lg:grid-cols-2">
                <!-- Queues -->
                <table class="w-full text-[11px]">
                    <thead>
                        <tr class="text-dim text-left">
                            <th class="pb-1.5 font-medium">Phase</th>
                            <th class="pb-1.5 text-right font-medium">Pending</th>
                            <th class="pb-1.5 text-right font-medium">Running</th>
                            <th class="pb-1.5 text-right font-medium">
                                Failed ({{ payload.failed_window_hours ?? 24 }}h)
                            </th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-if="phases.length === 0">
                            <td colspan="4" class="text-dim py-2">No active job.</td>
                        </tr>
                        <tr
                            v-for="phase in phases"
                            :key="phase.phase"
                            class="border-border border-t font-mono"
                        >
                            <td class="py-1.5 font-sans text-white/90">
                                {{ PHASE_LABELS[phase.phase] ?? phase.phase }}
                            </td>
                            <td class="py-1.5 text-right text-white">
                                {{ number.format(phase.pending) }}
                            </td>
                            <td
                                class="py-1.5 text-right"
                                :class="phase.running ? 'text-lava' : 'text-dim'"
                            >
                                {{ number.format(phase.running) }}
                            </td>
                            <td
                                class="py-1.5 text-right"
                                :class="phase.failed ? 'text-amber-400' : 'text-dim'"
                            >
                                {{ number.format(phase.failed) }}
                            </td>
                        </tr>
                    </tbody>
                </table>

                <!-- Coverage -->
                <div class="space-y-2">
                    <div v-for="item in coverageItems" :key="item.key">
                        <div class="mb-0.5 flex justify-between text-[11px]">
                            <span class="text-white/90">{{ item.label }}</span>
                            <span class="text-dim font-mono">
                                {{ number.format(item.count) }} /
                                {{ number.format(payload.coverage?.total ?? 0) }} ·
                                {{ item.pct.toFixed(item.pct < 10 ? 1 : 0) }}%
                            </span>
                        </div>
                        <div class="h-1 rounded-full bg-white/10">
                            <div
                                class="h-full rounded-full transition-all"
                                :class="item.pct >= 99.9 ? 'bg-emerald' : 'bg-lava'"
                                :style="{ width: `${item.pct}%` }"
                            />
                        </div>
                    </div>
                </div>
            </div>
        </template>
    </div>
</template>
