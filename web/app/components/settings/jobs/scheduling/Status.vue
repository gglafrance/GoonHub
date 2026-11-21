<script setup lang="ts">
import type { JobScheduleStatus, PhaseScheduleState } from '~/types/jobs';

const props = defineProps<{
    status: JobScheduleStatus;
}>();

const { phaseLabel, phaseIcon } = useJobFormatting();
const { formatDateTime } = useDateFormatter();

// Wall clock in the schedule timezone, e.g. "01:00", or "Sat 01:00" when not today.
const dayKey = (d: Date) =>
    new Intl.DateTimeFormat('en-CA', { timeZone: props.status.timezone }).format(d);

const clock = (iso: string): string => {
    const d = new Date(iso);
    const time = new Intl.DateTimeFormat('en-GB', {
        hour: '2-digit',
        minute: '2-digit',
        hourCycle: 'h23',
        timeZone: props.status.timezone,
    }).format(d);
    if (dayKey(d) === dayKey(new Date(props.status.now))) return time;
    const weekday = new Intl.DateTimeFormat('en-GB', {
        weekday: 'short',
        timeZone: props.status.timezone,
    }).format(d);
    return `${weekday} ${time}`;
};

const stateLabel = (p: PhaseScheduleState): string => {
    if (p.state === 'paused')
        return p.next_change ? `Paused until ${clock(p.next_change)}` : 'Paused';
    if (p.next_change) return `Running until ${clock(p.next_change)}`;
    return 'Running';
};

const workersLabel = (p: PhaseScheduleState): string => {
    const n = `${p.workers} worker${p.workers === 1 ? '' : 's'}`;
    if (p.quiet_workers === 0) return n;
    return `${n} (${p.worker_mode === 'quiet' ? 'quiet hours' : 'day'})`;
};

const quietLabel = computed(() => {
    const next = clock(props.status.quiet_hours_next_change);
    return props.status.quiet_hours ? `Quiet hours until ${next}` : `Quiet hours from ${next}`;
});

const capLabel = computed(() => {
    const s = props.status;
    if (s.max_concurrent_jobs === 0) return `${s.running_jobs} running · no global limit`;
    return `${s.running_jobs} / ${s.max_concurrent_jobs} running`;
});
</script>

<template>
    <div class="glass-panel p-5">
        <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
            <div>
                <h3 class="text-sm font-semibold text-white">Current State</h3>
                <p class="text-dim mt-1 text-[11px]">
                    {{ clock(status.now) }} in {{ status.timezone }} · {{ quietLabel }}
                </p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
                <span
                    class="rounded-full border border-white/10 bg-white/5 px-2.5 py-0.5 text-[10px]
                        text-white"
                >
                    {{ capLabel }}
                </span>
                <span
                    v-if="status.new_scene_since"
                    class="rounded-full border border-emerald-500/30 bg-emerald-500/10 px-2.5 py-0.5
                        text-[10px] text-emerald-400"
                    :title="`Scenes added since ${formatDateTime(status.new_scene_since)} go first`"
                >
                    New scenes first
                </span>
            </div>
        </div>

        <div class="divide-y divide-white/5">
            <div
                v-for="p in status.phases"
                :key="p.phase"
                class="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 py-2"
            >
                <div class="flex min-w-40 items-center gap-2">
                    <Icon :name="phaseIcon(p.phase)" size="13" class="text-dim" />
                    <span class="text-xs font-medium text-white">{{ phaseLabel(p.phase) }}</span>
                    <span class="text-dim text-[10px]">priority {{ p.priority }}</span>
                </div>
                <div class="flex flex-1 flex-wrap items-center justify-end gap-x-4 gap-y-1">
                    <span
                        class="flex items-center gap-1.5 text-[11px]"
                        :class="p.state === 'paused' ? 'text-lava' : 'text-emerald-400'"
                    >
                        <span
                            class="h-1.5 w-1.5 rounded-full"
                            :class="p.state === 'paused' ? 'bg-lava' : 'bg-emerald-400'"
                        />
                        {{ stateLabel(p) }}
                    </span>
                    <span class="w-36 text-right text-[11px] text-white">
                        {{ workersLabel(p) }}
                    </span>
                    <span class="text-dim w-48 text-right text-[10px] tabular-nums">
                        {{ p.pending.toLocaleString() }} pending · {{ p.queued }} queued ·
                        {{ p.running }} running
                    </span>
                </div>
            </div>
        </div>
    </div>
</template>
