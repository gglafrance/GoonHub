<script setup lang="ts">
import type { Relationship, RelationshipChange } from '~/types/social';

// Follow and friend buttons toward one user. Creating a relation needs social:use; undoing
// one (unfollow, cancel, decline, remove) is always possible.
const props = defineProps<{ userId: number; username: string; relationship: Relationship }>();
const emit = defineEmits<{ update: [relationship: Relationship] }>();

const api = useApiSocial();
const perms = useMyPermissions();
const socialStore = useSocialStore();

const busy = ref(false);
const error = ref('');
const confirmRemove = ref(false);

const canUse = computed(() => perms.has('social:use'));
const action = computed(() => friendAction(props.relationship));

async function run(fn: () => Promise<RelationshipChange>, touchesIncoming = false) {
    busy.value = true;
    error.value = '';
    try {
        const res = await fn();
        emit('update', res.relationship);
        if (touchesIncoming) socialStore.changed();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Action failed';
    } finally {
        busy.value = false;
    }
}

const toggleFollow = () =>
    run(() =>
        props.relationship.following ? api.unfollow(props.userId) : api.follow(props.userId),
    );
const addFriend = () =>
    run(() => api.sendFriendRequest(props.userId), props.relationship.request_received);
const cancelRequest = () => run(() => api.cancelFriendRequest(props.userId));
const accept = () => run(() => api.acceptFriendRequest(props.userId), true);
const decline = () => run(() => api.declineFriendRequest(props.userId), true);
const removeFriend = () => {
    confirmRemove.value = false;
    run(() => api.removeFriend(props.userId));
};

const btn =
    'flex items-center gap-1 rounded-md border px-2 py-1 text-[11px] font-medium transition-all disabled:cursor-not-allowed disabled:opacity-40';
const neutral = `${btn} border-border text-dim hover:border-lava/30 hover:text-white`;
const primary = `${btn} border-lava/40 bg-lava/10 text-lava hover:bg-lava/20`;
</script>

<template>
    <div v-if="!relationship.self" class="flex flex-col items-end gap-1">
        <div class="flex flex-wrap items-center justify-end gap-1.5">
            <button
                v-if="relationship.following || canUse"
                :class="relationship.following ? neutral : primary"
                :disabled="busy"
                :title="relationship.following ? `Unfollow ${username}` : `Follow ${username}`"
                @click="toggleFollow"
            >
                <Icon
                    :name="relationship.following ? 'heroicons:check' : 'heroicons:plus'"
                    size="12"
                />
                {{ relationship.following ? 'Following' : 'Follow' }}
            </button>

            <button
                v-if="action === 'add' && canUse"
                :class="primary"
                :disabled="busy"
                @click="addFriend"
            >
                <Icon name="heroicons:user-plus" size="12" />
                Add friend
            </button>
            <button
                v-else-if="action === 'cancel'"
                :class="neutral"
                :disabled="busy"
                @click="cancelRequest"
            >
                <Icon name="heroicons:x-mark" size="12" />
                Cancel request
            </button>
            <template v-else-if="action === 'respond'">
                <button v-if="canUse" :class="primary" :disabled="busy" @click="accept">
                    <Icon name="heroicons:check" size="12" />
                    Accept
                </button>
                <button :class="neutral" :disabled="busy" @click="decline">
                    <Icon name="heroicons:x-mark" size="12" />
                    Decline
                </button>
            </template>
            <button
                v-else-if="action === 'remove'"
                :class="neutral"
                :disabled="busy"
                @click="confirmRemove = true"
            >
                <Icon name="heroicons:user-minus" size="12" />
                Remove friend
            </button>
        </div>
        <p v-if="error" class="text-lava text-[10px]">{{ error }}</p>

        <UiConfirmModal
            :visible="confirmRemove"
            title="Remove friend"
            confirm-label="Remove"
            icon="heroicons:user-minus"
            @close="confirmRemove = false"
            @confirm="removeFriend"
        >
            Remove {{ username }} from your friends? Follows are kept.
        </UiConfirmModal>
    </div>
</template>
