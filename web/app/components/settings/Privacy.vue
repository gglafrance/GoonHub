<script setup lang="ts">
import type { Audience, FriendRequestPolicy, PrivacySettings, PrivacyUpdate } from '~/types/social';

// Privacy settings: who can see each item (catalog from the server, grouped) and who may send
// friend requests. Every change is saved at once.
const api = useApiSocial();
const { message, error, clearMessages } = useSettingsMessage();

const settings = ref<PrivacySettings | null>(null);
const loading = ref(true);
const saving = ref(false);

const groups = computed(() => groupPrivacyItems(settings.value?.items ?? []));
const policyOptions = computed(() =>
    (settings.value?.friend_request_policies ?? []).map((p) => ({
        value: p.value,
        label: p.label,
    })),
);
const policyDescription = computed(
    () =>
        settings.value?.friend_request_policies.find(
            (p) => p.value === settings.value?.friend_request_policy,
        )?.description ?? '',
);
const hasCustom = computed(() => settings.value?.items.some((i) => !i.is_default) ?? false);

async function load() {
    loading.value = true;
    try {
        settings.value = await api.getPrivacySettings();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load the privacy settings';
    } finally {
        loading.value = false;
    }
}

async function save(update: PrivacyUpdate) {
    clearMessages();
    saving.value = true;
    try {
        settings.value = await api.updatePrivacySettings(update);
        message.value = 'Privacy settings saved';
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save the privacy settings';
    } finally {
        saving.value = false;
    }
}

const setItem = (key: string, audience: Audience | 'default') =>
    save({ items: { [key]: audience } });
const setPolicy = (value: string | number) =>
    save({ friend_request_policy: String(value) as FriendRequestPolicy });
const resetAll = () =>
    save({
        items: Object.fromEntries((settings.value?.items ?? []).map((i) => [i.key, 'default'])),
    });

function onInvisibleSaved(text: string) {
    clearMessages();
    message.value = text;
}
function onInvisibleFailed(text: string) {
    clearMessages();
    error.value = text;
}

onMounted(load);
</script>

<template>
    <div class="space-y-6">
        <div
            v-if="message"
            class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2 text-xs"
        >
            {{ message }}
        </div>
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <div v-if="loading" class="flex justify-center py-8"><LoadingSpinner /></div>

        <template v-else-if="settings">
            <div class="glass-panel p-5">
                <div class="mb-1 flex items-center justify-between">
                    <h3 class="text-sm font-semibold text-white">Who can see what</h3>
                    <button
                        v-if="hasCustom"
                        class="text-dim hover:text-lava text-[11px] transition-colors
                            disabled:opacity-40"
                        :disabled="saving"
                        @click="resetAll"
                    >
                        Reset to defaults
                    </button>
                </div>
                <p class="text-dim mb-3 text-[11px]">
                    Friends see everything except "Only me". Mutuals also pass "Followers". Anyone
                    can follow you without approval, so "Followers" is close to everyone; use
                    Mutuals or Friends to restrict. These settings apply to administrators too.
                </p>
                <div v-for="g in groups" :key="g.group" class="mt-4 first:mt-0">
                    <h4 class="text-dim text-[11px] font-medium tracking-wider uppercase">
                        {{ g.group }}
                    </h4>
                    <div class="divide-border divide-y">
                        <SettingsPrivacyInvisibleToggle
                            v-if="g.group === 'Presence'"
                            @saved="onInvisibleSaved"
                            @failed="onInvisibleFailed"
                        />
                        <SettingsPrivacyAudienceRow
                            v-for="item in g.items"
                            :key="item.key"
                            :item="item"
                            :audiences="settings.audiences"
                            :disabled="saving"
                            @change="setItem(item.key, $event)"
                        />
                    </div>
                </div>
            </div>

            <div class="glass-panel p-5">
                <h3 class="mb-1 text-sm font-semibold text-white">Friend requests</h3>
                <p class="text-dim mb-3 text-[11px]">
                    Who can send you a friend request. You can always accept a request you received
                    before changing this.
                </p>
                <div class="sm:w-64" :class="saving ? 'pointer-events-none opacity-50' : ''">
                    <UiSelectMenu
                        :model-value="settings.friend_request_policy"
                        :options="policyOptions"
                        @update:model-value="setPolicy"
                    />
                </div>
                <p class="text-dim mt-2 text-[11px]">{{ policyDescription }}</p>
            </div>

            <SettingsPrivacyMessages @saved="onInvisibleSaved" @failed="onInvisibleFailed" />
        </template>
    </div>
</template>
