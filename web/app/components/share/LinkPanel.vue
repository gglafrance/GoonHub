<script setup lang="ts">
import type { ShareLink } from '~/types/share';

// "Link" of the Share dialog: the share links of the scene (mine) and a new one, public or
// for signed-in users, with an expiry. It starts at `start` (seconds, 0: the beginning) or
// plays the range of a marker (markerId).
const props = defineProps<{ sceneId: number; start?: number; markerId?: number }>();

const { createShareLink, listShareLinks, deleteShareLink } = useApiShares();

const links = ref<ShareLink[]>([]);
const shareBaseUrl = ref('');
const loading = ref(false);
const creating = ref(false);
const error = ref('');

const shareType = ref<'public' | 'auth_required'>('public');
const expiresIn = ref('7d');

const expiryOptions = [
    { value: '1h', label: '1 hour' },
    { value: '24h', label: '24 hours' },
    { value: '7d', label: '7 days' },
    { value: '30d', label: '30 days' },
    { value: 'never', label: 'Never' },
];

const createLabel = computed(() => {
    if (props.markerId) return 'Create a link to this marker';
    if (props.start) return `Create a link starting at ${formatClock(props.start)}`;
    return 'Create share link';
});

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
        await createShareLink(
            props.sceneId,
            shareType.value,
            expiresIn.value,
            props.markerId
                ? { marker_id: props.markerId }
                : props.start
                  ? { start: props.start }
                  : {},
        );
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
        // the link stays listed
    }
};

onMounted(loadLinks);
</script>

<template>
    <div>
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

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
            <div v-else class="text-dim/50 mb-4 py-4 text-center text-xs">No share links yet</div>
        </div>

        <div class="border-border space-y-3 border-t pt-4">
            <span class="text-dim text-[10px] font-medium tracking-wider uppercase">
                Create New Link
            </span>
            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
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
            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
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
            <button
                :disabled="creating"
                class="bg-lava hover:bg-lava-glow w-full rounded-lg px-4 py-2 text-xs font-semibold
                    text-white transition-all disabled:cursor-not-allowed disabled:opacity-40"
                @click="handleCreate"
            >
                {{ creating ? 'Creating...' : createLabel }}
            </button>
        </div>
    </div>
</template>
