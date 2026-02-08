<script setup lang="ts">
import type { ShareLink } from '~/types/share';

const props = defineProps<{
    link: ShareLink;
    baseUrl?: string;
}>();

const emit = defineEmits<{
    delete: [id: number];
}>();

const copied = ref(false);
const deleting = ref(false);

const shareUrl = computed(() => {
    // Use share domain for public links, main domain for auth_required
    const origin =
        props.link.share_type === 'public' && props.baseUrl
            ? props.baseUrl
            : window.location.origin;
    return `${origin}/share/${props.link.token}`;
});

const isExpired = computed(() => {
    if (!props.link.expires_at) return false;
    return new Date(props.link.expires_at) < new Date();
});

const expiryLabel = computed(() => {
    if (!props.link.expires_at) return 'Never';
    if (isExpired.value) return 'Expired';
    const diff = new Date(props.link.expires_at).getTime() - Date.now();
    const hours = Math.floor(diff / (1000 * 60 * 60));
    const days = Math.floor(hours / 24);
    if (days > 0) return `${days}d left`;
    if (hours > 0) return `${hours}h left`;
    const minutes = Math.floor(diff / (1000 * 60));
    return `${minutes}m left`;
});

const handleCopy = async () => {
    try {
        await navigator.clipboard.writeText(shareUrl.value);
        copied.value = true;
        setTimeout(() => {
            copied.value = false;
        }, 2000);
    } catch {
        // Fallback
    }
};

const handleDelete = () => {
    deleting.value = true;
    emit('delete', props.link.id);
};
</script>

<template>
    <div
        class="border-border/50 bg-panel/30 group flex items-center gap-3 rounded-lg border px-3
            py-2.5"
        :class="{ 'opacity-50': isExpired }"
    >
        <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
                <span class="truncate font-mono text-[11px] text-white/70">
                    {{ link.token }}
                </span>
                <span
                    class="shrink-0 rounded px-1.5 py-px text-[9px] font-semibold tracking-wide
                        uppercase"
                    :class="
                        link.share_type === 'public'
                            ? 'border border-emerald-500/20 bg-emerald-500/10 text-emerald-400'
                            : 'border border-amber-500/20 bg-amber-500/10 text-amber-400'
                    "
                >
                    {{ link.share_type === 'public' ? 'Public' : 'Auth' }}
                </span>
            </div>
            <div class="text-dim mt-1 flex items-center gap-3 text-[10px]">
                <span class="flex items-center gap-1">
                    <Icon name="heroicons:eye" size="10" />
                    {{ link.view_count }}
                </span>
                <span class="flex items-center gap-1">
                    <Icon name="heroicons:clock" size="10" />
                    {{ expiryLabel }}
                </span>
            </div>
        </div>

        <div class="flex shrink-0 items-center gap-1">
            <button
                class="text-dim rounded p-1.5 transition-colors hover:text-emerald-400"
                :title="copied ? 'Copied!' : 'Copy link'"
                @click="handleCopy"
            >
                <Icon
                    :name="copied ? 'heroicons:check' : 'heroicons:clipboard-document'"
                    size="14"
                />
            </button>
            <button
                class="text-dim hover:text-lava rounded p-1.5 transition-colors"
                title="Delete link"
                :disabled="deleting"
                @click="handleDelete"
            >
                <Icon name="heroicons:trash" size="14" />
            </button>
        </div>
    </div>
</template>
