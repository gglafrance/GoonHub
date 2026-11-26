<script setup lang="ts">
import type { ImageCrop, ProfileEdit } from '~/types/profile';

// Avatar and banner of the owner's profile: upload (cropped in the browser, processed by the
// server), remove, or a favorite video's thumbnail as the banner.
const props = defineProps<{ state: ProfileEdit }>();
const emit = defineEmits<{ changed: [] }>();

const api = useApiProfiles();
const socialStore = useSocialStore();
const error = ref('');
const busy = ref(false);
const cropping = ref<{ kind: 'avatar' | 'banner'; file: File } | null>(null);
const fileInput = ref<HTMLInputElement | null>(null);
let pickKind: 'avatar' | 'banner' = 'avatar';

const maxMB = computed(() => Math.round(props.state.limits.max_upload_bytes / 1048576));

function pick(kind: 'avatar' | 'banner') {
    pickKind = kind;
    fileInput.value?.click();
}

function onFile(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) return;
    if (file.size > props.state.limits.max_upload_bytes) {
        error.value = `The image is larger than ${maxMB.value} MB`;
        return;
    }
    error.value = '';
    cropping.value = { kind: pickKind, file };
}

async function run(action: () => Promise<unknown>) {
    busy.value = true;
    error.value = '';
    try {
        await action();
        emit('changed');
        socialStore.refresh();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'The change failed';
    } finally {
        busy.value = false;
    }
}

function onCrop(crop: ImageCrop) {
    const job = cropping.value;
    cropping.value = null;
    if (!job) return;
    run(() =>
        job.kind === 'avatar' ? api.uploadAvatar(job.file, crop) : api.uploadBanner(job.file, crop),
    );
}

const buttonClass =
    'border-border hover:border-lava/40 rounded-md border px-2.5 py-1 text-[11px] text-white transition-colors disabled:opacity-50';
</script>

<template>
    <div class="space-y-4">
        <input
            ref="fileInput"
            type="file"
            accept="image/jpeg,image/png,image/gif"
            hidden
            @change="onFile"
        />
        <div class="flex items-center gap-3">
            <SocialUserAvatar
                :username="state.user.username"
                :display-name="state.display_name"
                :user-id="state.user.id"
                :avatar-version="state.avatar_version"
                size="lg"
            />
            <div class="space-y-1.5">
                <div class="text-[11px] font-medium text-white">Avatar</div>
                <div class="flex gap-2">
                    <button :class="buttonClass" :disabled="busy" @click="pick('avatar')">
                        Upload
                    </button>
                    <button
                        v-if="state.avatar_version"
                        :class="buttonClass"
                        :disabled="busy"
                        @click="run(api.deleteAvatar)"
                    >
                        Remove
                    </button>
                </div>
            </div>
        </div>
        <div class="space-y-1.5">
            <div class="text-[11px] font-medium text-white">Banner</div>
            <div
                class="border-border aspect-[3/1] w-full max-w-md overflow-hidden rounded-lg border
                    bg-white/[0.02]"
            >
                <img
                    v-if="state.banner.url"
                    :src="state.banner.url"
                    alt=""
                    class="h-full w-full object-cover"
                />
            </div>
            <div class="flex flex-wrap gap-2">
                <button :class="buttonClass" :disabled="busy" @click="pick('banner')">
                    Upload
                </button>
                <button
                    v-if="state.banner.type !== 'none'"
                    :class="buttonClass"
                    :disabled="busy"
                    @click="run(api.clearBanner)"
                >
                    Remove
                </button>
            </div>
            <div v-if="state.favorite_scenes.length" class="space-y-1">
                <div class="text-dim text-[10px]">Or use a favorite video:</div>
                <div class="flex flex-wrap gap-2">
                    <button
                        v-for="scene in state.favorite_scenes"
                        :key="scene.id"
                        class="border-border hover:border-lava/50 h-10 w-18 overflow-hidden rounded
                            border transition-colors"
                        :class="state.banner.scene_id === scene.id ? 'border-lava' : ''"
                        :title="scene.title"
                        :disabled="busy"
                        @click="run(() => api.setBannerScene(scene.id))"
                    >
                        <img
                            :src="`/thumbnails/${scene.id}?size=sm`"
                            alt=""
                            class="h-full w-full object-cover"
                        />
                    </button>
                </div>
            </div>
        </div>
        <p class="text-dim text-[10px]">JPEG, PNG or GIF, up to {{ maxMB }} MB.</p>
        <p v-if="error" class="text-lava text-[11px]">{{ error }}</p>
        <ProfileEditImageCropper
            :visible="!!cropping"
            :file="cropping?.file ?? null"
            :aspect="cropping?.kind === 'banner' ? 3 : 1"
            :title="cropping?.kind === 'banner' ? 'Crop the banner' : 'Crop the avatar'"
            @close="cropping = null"
            @confirm="onCrop"
        />
    </div>
</template>
