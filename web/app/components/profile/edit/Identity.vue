<script setup lang="ts">
import type { ProfileEdit } from '~/types/profile';

// Display name and bio of the owner's profile (plain text; links are made clickable).
const props = defineProps<{ state: ProfileEdit }>();
const emit = defineEmits<{ saved: [state: ProfileEdit] }>();

const api = useApiProfiles();
const displayName = ref('');
const bio = ref('');
const saving = ref(false);
const error = ref('');
const done = ref(false);

function sync() {
    displayName.value = props.state.display_name;
    bio.value = props.state.bio;
}
watch(() => props.state, sync);
onMounted(sync);

const dirty = computed(
    () => displayName.value !== props.state.display_name || bio.value !== props.state.bio,
);
const bioLength = computed(() => Array.from(bio.value).length);

async function save() {
    saving.value = true;
    error.value = '';
    done.value = false;
    try {
        const state = await api.updateMyProfile({
            display_name: displayName.value,
            bio: bio.value,
        });
        done.value = true;
        emit('saved', state);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save the profile';
    } finally {
        saving.value = false;
    }
}
</script>

<template>
    <div class="space-y-3">
        <label class="block">
            <span class="text-dim mb-1 block text-[11px]">Display name</span>
            <input
                v-model="displayName"
                type="text"
                :maxlength="state.limits.display_name_max_length"
                :placeholder="state.user.username"
                class="border-border focus:border-lava/50 w-full rounded-md border bg-white/[0.03]
                    px-3 py-1.5 text-xs text-white outline-none"
            />
        </label>
        <label class="block">
            <span class="text-dim mb-1 flex justify-between text-[11px]">
                <span>Bio</span>
                <span
                    class="font-mono"
                    :class="bioLength > state.limits.bio_max_length ? 'text-lava' : ''"
                >
                    {{ bioLength }}/{{ state.limits.bio_max_length }}
                </span>
            </span>
            <textarea
                v-model="bio"
                rows="4"
                placeholder="A few words about you. Links (https://...) are clickable."
                class="border-border focus:border-lava/50 w-full resize-y rounded-md border
                    bg-white/[0.03] px-3 py-2 text-xs text-white outline-none"
            />
        </label>
        <div class="flex items-center gap-3">
            <button
                class="bg-lava hover:bg-lava/90 rounded-md px-3 py-1.5 text-xs font-medium
                    text-white disabled:opacity-50"
                :disabled="!dirty || saving || bioLength > state.limits.bio_max_length"
                @click="save"
            >
                {{ saving ? 'Saving...' : 'Save' }}
            </button>
            <span v-if="done && !dirty" class="text-emerald text-[11px]">Saved</span>
            <span v-if="error" class="text-lava text-[11px]">{{ error }}</span>
        </div>
    </div>
</template>
