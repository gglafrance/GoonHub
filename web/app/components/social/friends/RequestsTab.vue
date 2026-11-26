<script setup lang="ts">
// Pending friend requests: received (accept / decline) and sent (cancel). Reloaded on every
// social event of the signed-in user.
const api = useApiSocial();
const socialStore = useSocialStore();

const loadIncoming = (page: number) => api.listFriendRequests('incoming', page);
const loadOutgoing = (page: number) => api.listFriendRequests('outgoing', page);
</script>

<template>
    <div class="space-y-6">
        <section>
            <h3 class="text-dim mb-2 text-[11px] font-medium tracking-wider uppercase">Received</h3>
            <SocialUserList
                :loader="loadIncoming"
                :reload-key="socialStore.version"
                :keep-if="(rel) => rel.request_received"
                since-label="Sent"
                empty-text="No pending request."
            />
        </section>
        <section>
            <h3 class="text-dim mb-2 text-[11px] font-medium tracking-wider uppercase">Sent</h3>
            <SocialUserList
                :loader="loadOutgoing"
                :reload-key="socialStore.version"
                :keep-if="(rel) => rel.request_sent"
                since-label="Sent"
                empty-text="You have no pending request."
            />
        </section>
    </div>
</template>
