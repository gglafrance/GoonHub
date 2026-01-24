<script setup lang="ts">
import type { Video } from '~/types/video';
import type { Tag } from '~/types/tag';

const video = inject<Ref<Video | null>>('watchVideo');
const { fetchTags, fetchVideoTags, setVideoTags, updateVideoDetails } = useApi();

const loading = ref(false);
const error = ref<string | null>(null);

const allTags = ref<Tag[]>([]);
const videoTags = ref<Tag[]>([]);
const showTagPicker = ref(false);

const anchorRef = ref<HTMLElement | null>(null);

const editingTitle = ref(false);
const editingDescription = ref(false);
const editTitle = ref('');
const editDescription = ref('');
const saving = ref(false);
const saved = ref(false);
let savedTimeout: ReturnType<typeof setTimeout> | null = null;

const titleInputRef = ref<HTMLInputElement | null>(null);
const descriptionInputRef = ref<HTMLTextAreaElement | null>(null);

const availableTags = computed(() =>
    allTags.value.filter((t) => !videoTags.value.some((vt) => vt.id === t.id)),
);

onMounted(async () => {
    await loadTags();
});

function startEditTitle() {
    editTitle.value = video?.value?.title || '';
    editingTitle.value = true;
    nextTick(() => titleInputRef.value?.focus());
}

function startEditDescription() {
    editDescription.value = video?.value?.description || '';
    editingDescription.value = true;
    nextTick(() => {
        if (descriptionInputRef.value) {
            descriptionInputRef.value.focus();
            autoResize({ target: descriptionInputRef.value } as unknown as Event);
        }
    });
}

async function saveTitle() {
    editingTitle.value = false;
    if (!video?.value) return;
    if (editTitle.value === (video.value.title || '')) return;
    await saveDetails(editTitle.value, video.value.description || '');
}

async function saveDescription() {
    editingDescription.value = false;
    if (!video?.value) return;
    if (editDescription.value === (video.value.description || '')) return;
    await saveDetails(video.value.title || '', editDescription.value);
}

async function saveDetails(title: string, description: string) {
    if (!video?.value) return;

    saving.value = true;
    error.value = null;

    try {
        const updated = await updateVideoDetails(video.value.id, title, description);
        if (video.value) {
            video.value.title = updated.title;
            video.value.description = updated.description;
        }
        saved.value = true;
        if (savedTimeout) clearTimeout(savedTimeout);
        savedTimeout = setTimeout(() => {
            saved.value = false;
        }, 2000);
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to save details';
    } finally {
        saving.value = false;
    }
}

function autoResize(event: Event) {
    const el = event.target as HTMLTextAreaElement;
    el.style.height = 'auto';
    el.style.height = el.scrollHeight + 'px';
}

async function loadTags() {
    if (!video?.value) return;
    loading.value = true;
    error.value = null;

    try {
        const [tagsRes, videoTagsRes] = await Promise.all([
            fetchTags(),
            fetchVideoTags(video.value.id),
        ]);
        allTags.value = tagsRes.data || [];
        videoTags.value = videoTagsRes.data || [];
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to load tags';
    } finally {
        loading.value = false;
    }
}

async function addTag(tagId: number) {
    if (!video?.value) return;
    error.value = null;

    const newIds = [...videoTags.value.map((t) => t.id), tagId];

    try {
        const res = await setVideoTags(video.value.id, newIds);
        videoTags.value = res.data || [];
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to update tags';
    }
}

async function removeTag(tagId: number) {
    if (!video?.value) return;
    error.value = null;

    const newIds = videoTags.value.filter((t) => t.id !== tagId).map((t) => t.id);

    try {
        const res = await setVideoTags(video.value.id, newIds);
        videoTags.value = res.data || [];
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to update tags';
    }
}
</script>

<template>
    <div class="min-h-64 space-y-4">
        <!-- Error -->
        <div
            v-if="error"
            class="border-lava/30 bg-lava/5 flex items-center gap-2 rounded-lg border px-3 py-2"
        >
            <Icon name="heroicons:exclamation-triangle" size="14" class="text-lava" />
            <span class="text-xs text-red-300">{{ error }}</span>
        </div>

        <!-- Title -->
        <div class="space-y-1">
            <div class="flex items-center gap-2">
                <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">Title</h3>
                <Transition name="fade">
                    <span v-if="saved" class="text-[10px] text-emerald-400/80">Saved</span>
                </Transition>
            </div>

            <input
                v-if="editingTitle"
                ref="titleInputRef"
                v-model="editTitle"
                @blur="saveTitle"
                @keydown.enter="($event.target as HTMLInputElement).blur()"
                type="text"
                class="border-border focus:border-lava/50 -mx-2 w-[calc(100%+16px)] rounded-md
                    border bg-white/3 px-2 py-1 text-sm text-white transition-colors outline-none"
            />
            <p
                v-else
                @click="startEditTitle"
                class="text-dim -mx-2 cursor-pointer rounded-md px-2 py-1 text-sm transition-colors
                    hover:bg-white/3 hover:text-white"
                :class="{ 'text-white': video?.title }"
            >
                {{ video?.title || 'Untitled' }}
            </p>
        </div>

        <!-- Description -->
        <div class="space-y-1">
            <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">Description</h3>

            <textarea
                v-if="editingDescription"
                ref="descriptionInputRef"
                v-model="editDescription"
                @blur="saveDescription"
                @input="autoResize"
                rows="2"
                class="border-border focus:border-lava/50 -mx-2 w-[calc(100%+16px)] resize-none
                    rounded-md border bg-white/3 px-2 py-1 text-sm text-white transition-colors
                    outline-none"
            />
            <p
                v-else
                @click="startEditDescription"
                class="text-dim -mx-2 cursor-pointer rounded-md px-2 py-1 text-sm
                    whitespace-pre-wrap transition-colors hover:bg-white/3 hover:text-white"
                :class="{ 'text-white/70': video?.description }"
            >
                {{ video?.description || 'No description' }}
            </p>
        </div>

        <!-- Tags section -->
        <div class="space-y-2">
            <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">Tags</h3>

            <div v-if="loading" class="flex items-center gap-2 py-2">
                <LoadingSpinner />
            </div>

            <div v-else class="flex flex-wrap items-center gap-1.5">
                <!-- Applied tags -->
                <span
                    v-for="tag in videoTags"
                    :key="tag.id"
                    class="group flex items-center gap-1.5 rounded-full border px-2.5 py-0.5
                        text-[11px] font-medium text-white"
                    :style="{
                        borderColor: tag.color + '60',
                        backgroundColor: tag.color + '15',
                    }"
                >
                    <span
                        class="inline-block h-2 w-2 rounded-full"
                        :style="{ backgroundColor: tag.color }"
                    />
                    {{ tag.name }}
                    <span
                        @click="removeTag(tag.id)"
                        class="cursor-pointer opacity-0 transition-opacity group-hover:opacity-60
                            hover:opacity-100!"
                    >
                        <Icon name="heroicons:x-mark" size="10" />
                    </span>
                </span>

                <!-- Add tag button -->
                <button
                    ref="anchorRef"
                    @click="showTagPicker = !showTagPicker"
                    class="border-border hover:border-border-hover flex h-5 w-5 items-center
                        justify-center rounded-full border transition-colors"
                    title="Add tag"
                >
                    <Icon name="heroicons:plus" size="12" class="text-dim" />
                </button>

                <WatchTagPicker
                    :visible="showTagPicker"
                    :tags="availableTags"
                    :anchor-el="anchorRef"
                    @select="addTag"
                    @close="showTagPicker = false"
                />
            </div>
        </div>
    </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
    transition: opacity 0.3s ease;
}
.fade-enter-from,
.fade-leave-to {
    opacity: 0;
}
</style>
