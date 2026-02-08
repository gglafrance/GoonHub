<script setup lang="ts">
import type { ShareLink } from '~/types/share';

const props = defineProps<{
    visible: boolean;
    sceneId: number;
}>();

const emit = defineEmits<{
    close: [];
}>();

const { createShareLink, listShareLinks, deleteShareLink } = useApiShares();

const links = ref<ShareLink[]>([]);
const shareBaseUrl = ref('');
const loading = ref(false);
const creating = ref(false);
const error = ref('');

// Form state
const shareType = ref<'public' | 'auth_required'>('public');
const expiresIn = ref('7d');

const expiryOptions = [
    { value: '1h', label: '1 hour' },
    { value: '24h', label: '24 hours' },
    { value: '7d', label: '7 days' },
    { value: '30d', label: '30 days' },
    { value: 'never', label: 'Never' },
];

const loadLinks = async () => {
    loading.value = true;
    try {
        const data = await listShareLinks(props.sceneId);
        links.value = data.share_links || [];
        shareBaseUrl.value = data.share_base_url || '';
    } catch {
        links.value = [];
    } finally {
        loading.value = false;
    }
};

const handleCreate = async () => {
    error.value = '';
    creating.value = true;
    try {
        await createShareLink(props.sceneId, shareType.value, expiresIn.value);
        await loadLinks();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to create share link';
    } finally {
        creating.value = false;
    }
};

const handleDelete = async (linkId: number) => {
    try {
        await deleteShareLink(linkId);
        links.value = links.value.filter((l) => l.id !== linkId);
    } catch {
        // Silent fail
    }
};

const handleClose = () => {
    error.value = '';
    emit('close');
};

watch(
    () => props.visible,
    (val) => {
        if (val) loadLinks();
    },
);
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div class="glass-panel border-border w-full max-w-md border p-6">
                <div class="mb-4 flex items-center justify-between">
                    <h3 class="text-sm font-semibold text-white">Share Scene</h3>
                    <button
                        class="text-dim transition-colors hover:text-white"
                        @click="handleClose"
                    >
                        <Icon name="heroicons:x-mark" size="16" />
                    </button>
                </div>

                <!-- Error -->
                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>

                <!-- Existing Links -->
                <div v-if="loading" class="flex justify-center py-6">
                    <LoadingSpinner label="Loading..." />
                </div>

                <div v-else>
                    <div v-if="links.length > 0" class="mb-4 space-y-2">
                        <span class="text-dim text-[10px] font-medium tracking-wider uppercase">
                            Active Links
                        </span>
                        <div class="max-h-48 space-y-1.5 overflow-y-auto">
                            <ShareLinkItem
                                v-for="link in links"
                                :key="link.id"
                                :link="link"
                                :base-url="shareBaseUrl"
                                @delete="handleDelete"
                            />
                        </div>
                    </div>

                    <div v-else class="text-dim/50 mb-4 py-4 text-center text-xs">
                        No share links yet
                    </div>
                </div>

                <!-- Create Form -->
                <div class="border-border space-y-3 border-t pt-4">
                    <span class="text-dim text-[10px] font-medium tracking-wider uppercase">
                        Create New Link
                    </span>

                    <!-- Share Type Toggle -->
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Visibility
                        </label>
                        <div class="flex gap-2">
                            <button
                                class="flex-1 rounded-lg border px-3 py-2 text-xs font-medium
                                    transition-all"
                                :class="
                                    shareType === 'public'
                                        ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-400'
                                        : `border-border text-dim hover:border-border-hover
                                            hover:text-white`
                                "
                                @click="shareType = 'public'"
                            >
                                <Icon name="heroicons:globe-alt" size="12" class="mr-1" />
                                Public
                            </button>
                            <button
                                class="flex-1 rounded-lg border px-3 py-2 text-xs font-medium
                                    transition-all"
                                :class="
                                    shareType === 'auth_required'
                                        ? 'border-amber-500/40 bg-amber-500/10 text-amber-400'
                                        : `border-border text-dim hover:border-border-hover
                                            hover:text-white`
                                "
                                @click="shareType = 'auth_required'"
                            >
                                <Icon name="heroicons:lock-closed" size="12" class="mr-1" />
                                Auth Required
                            </button>
                        </div>
                    </div>

                    <!-- Expiry -->
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Expires
                        </label>
                        <div class="flex flex-wrap gap-1.5">
                            <button
                                v-for="opt in expiryOptions"
                                :key="opt.value"
                                class="rounded-md border px-2.5 py-1.5 text-[11px] font-medium
                                    transition-all"
                                :class="
                                    expiresIn === opt.value
                                        ? 'border-lava/40 bg-lava/10 text-lava'
                                        : `border-border text-dim hover:border-border-hover
                                            hover:text-white`
                                "
                                @click="expiresIn = opt.value"
                            >
                                {{ opt.label }}
                            </button>
                        </div>
                    </div>

                    <!-- Create Button -->
                    <button
                        :disabled="creating"
                        class="bg-lava hover:bg-lava-glow w-full rounded-lg px-4 py-2 text-xs
                            font-semibold text-white transition-all disabled:cursor-not-allowed
                            disabled:opacity-40"
                        @click="handleCreate"
                    >
                        {{ creating ? 'Creating...' : 'Create Share Link' }}
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
