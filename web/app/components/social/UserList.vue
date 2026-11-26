<script setup lang="ts">
import type { Relationship, SocialUserEntry, SocialUserPage } from '~/types/social';

// A paginated list of users ("load more"). `keepIf` drops a row whose relation no longer
// belongs to the list (e.g. an accepted request); `reloadKey` reloads from the first page.
const props = withDefaults(
    defineProps<{
        loader: (page: number) => Promise<SocialUserPage>;
        emptyText: string;
        sinceLabel?: string;
        keepIf?: (relationship: Relationship) => boolean;
        reloadKey?: string | number;
    }>(),
    { sinceLabel: undefined, keepIf: undefined, reloadKey: 0 },
);

const entries = ref<SocialUserEntry[]>([]);
const total = ref(0);
const page = ref(1);
const totalPages = ref(0);
const loading = ref(false);
const error = ref('');
const restricted = ref(false);
let seq = 0;

async function load(next = 1) {
    const mySeq = ++seq;
    loading.value = true;
    error.value = '';
    try {
        const res = await props.loader(next);
        if (mySeq !== seq) return;
        entries.value = next === 1 ? res.data : [...entries.value, ...res.data];
        total.value = res.pagination.total_items;
        totalPages.value = res.pagination.total_pages;
        page.value = next;
        restricted.value = false;
    } catch (e: unknown) {
        if (mySeq !== seq) return;
        const status = (e as { status?: number }).status;
        restricted.value = status === 403;
        error.value = restricted.value
            ? 'This list is private.'
            : e instanceof Error
              ? e.message
              : 'Failed to load';
    } finally {
        if (mySeq === seq) loading.value = false;
    }
}

function onUpdate(entry: SocialUserEntry, relationship: Relationship) {
    if (props.keepIf && !props.keepIf(relationship)) {
        entries.value = entries.value.filter((e) => e.id !== entry.id);
        total.value = Math.max(0, total.value - 1);
        return;
    }
    entry.relationship = relationship;
}

watch(
    () => props.reloadKey,
    () => load(1),
);
onMounted(() => load(1));

defineExpose({ reload: () => load(1) });
</script>

<template>
    <div class="space-y-2">
        <div
            v-if="error"
            class="rounded-lg border px-3 py-2 text-xs"
            :class="
                restricted
                    ? 'border-border text-dim bg-white/[0.02]'
                    : 'border-lava/20 bg-lava/5 text-lava'
            "
        >
            <Icon v-if="restricted" name="heroicons:lock-closed" size="12" class="mr-1" />
            {{ error }}
        </div>
        <template v-else>
            <SocialUserRow
                v-for="entry in entries"
                :key="entry.id"
                :entry="entry"
                :since-label="sinceLabel"
                @update="onUpdate(entry, $event)"
            />
            <p
                v-if="!loading && entries.length === 0"
                class="text-dim border-border rounded-lg border border-dashed px-3 py-6 text-center
                    text-xs"
            >
                {{ emptyText }}
            </p>
            <div v-if="loading" class="flex justify-center py-4">
                <LoadingSpinner />
            </div>
            <button
                v-else-if="page < totalPages"
                class="border-border text-dim hover:border-lava/30 w-full rounded-lg border py-1.5
                    text-[11px] transition-all hover:text-white"
                @click="load(page + 1)"
            >
                Load more ({{ total - entries.length }} left)
            </button>
        </template>
    </div>
</template>
