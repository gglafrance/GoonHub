<script setup lang="ts">
import type { ShareCandidate } from '~/utils/messages';

// Pick users to write to: recent conversations first, then friends, then (when searching) the
// other users whose message settings let me write to them. Presence dots; users who do not
// accept my messages are shown disabled. Multi-select (v-model) or single (emits pick).
const props = defineProps<{ single?: boolean }>();
const selected = defineModel<number[]>({ default: () => [] });
const emit = defineEmits<{ pick: [user: ShareCandidate] }>();

const authStore = useAuthStore();
const messagesStore = useMessagesStore();
const social = useApiSocial();

const query = ref('');
const friends = ref<ShareCandidate[]>([]);
const found = ref<ShareCandidate[]>([]);
const loading = ref(true);
let searchTimer: ReturnType<typeof setTimeout> | null = null;

const toCandidate = (u: {
    id: number;
    username: string;
    display_name?: string;
    avatar_version?: number;
    relationship: { can_message?: boolean };
}): ShareCandidate => ({
    id: u.id,
    username: u.username,
    display_name: u.display_name ?? '',
    avatar_version: u.avatar_version ?? 0,
    can_message: !!u.relationship.can_message,
    recent: false,
});

const candidates = computed(() => {
    const base = shareCandidates(messagesStore.conversations, friends.value, query.value);
    const seen = new Set(base.map((c) => c.id));
    return [...base, ...found.value.filter((c) => !seen.has(c.id))];
});

onMounted(async () => {
    if (!messagesStore.loaded) messagesStore.refresh();
    const me = authStore.user?.username;
    try {
        if (me) {
            const page = await social.listRelations(me, 'friends', 1, 100);
            friends.value = page.data.map(toCandidate);
        }
    } catch {
        friends.value = [];
    } finally {
        loading.value = false;
    }
});

watch(query, (q) => {
    if (searchTimer) clearTimeout(searchTimer);
    found.value = [];
    if (q.trim().length < 2) return;
    searchTimer = setTimeout(async () => {
        try {
            const page = await social.searchUsers(q.trim(), 1, 20);
            found.value = page.data.map(toCandidate).filter((c) => c.can_message);
        } catch {
            found.value = [];
        }
    }, 250);
});

const toggle = (c: ShareCandidate) => {
    if (!c.can_message) return;
    if (props.single) {
        emit('pick', c);
        return;
    }
    selected.value = selected.value.includes(c.id)
        ? selected.value.filter((id) => id !== c.id)
        : [...selected.value, c.id];
};
</script>

<template>
    <div>
        <input
            v-model="query"
            type="search"
            placeholder="Search friends and users"
            class="border-border bg-surface focus:border-lava/40 mb-1.5 w-full rounded-md border
                px-2.5 py-1.5 text-xs text-white outline-none placeholder:text-white/30"
        />
        <div class="max-h-52 overflow-y-auto">
            <div v-if="loading" class="flex justify-center py-4"><LoadingSpinner /></div>
            <p v-else-if="candidates.length === 0" class="text-dim py-4 text-center text-[11px]">
                {{ query ? 'Nobody found.' : 'No friend yet: add some from the Friends page.' }}
            </p>
            <button
                v-for="c in candidates"
                :key="c.id"
                type="button"
                class="flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left
                    transition-colors"
                :class="[
                    c.can_message ? 'hover:bg-white/[0.05]' : 'cursor-not-allowed opacity-40',
                    selected.includes(c.id) ? 'bg-lava/10' : '',
                ]"
                :disabled="!c.can_message"
                :title="c.can_message ? undefined : `${c.username} does not accept your messages`"
                @click="toggle(c)"
            >
                <span class="relative shrink-0">
                    <SocialUserAvatar
                        :username="c.username"
                        :display-name="c.display_name"
                        :user-id="c.id"
                        :avatar-version="c.avatar_version"
                        size="xs"
                    />
                    <SocialPresenceDot :user-id="c.id" />
                </span>
                <span class="min-w-0 flex-1 truncate text-xs text-white">
                    {{ c.display_name || c.username }}
                    <span v-if="c.display_name" class="text-dim font-mono text-[10px]">
                        @{{ c.username }}
                    </span>
                </span>
                <span v-if="c.recent" class="text-dim text-[9px] tracking-wider uppercase"
                    >Recent</span
                >
                <Icon
                    v-if="!single"
                    :name="
                        selected.includes(c.id)
                            ? 'heroicons:check-circle-solid'
                            : 'heroicons:plus-circle'
                    "
                    size="15"
                    :class="selected.includes(c.id) ? 'text-lava' : 'text-dim'"
                />
            </button>
        </div>
    </div>
</template>
