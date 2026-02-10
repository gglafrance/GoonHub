<script setup lang="ts">
type FormState = {
    name: string;
    aliases: string[];
    image_url: string;
    gender: string;
    birthday: string;
    date_of_death: string;
    astrology: string;
    birthplace: string;
    ethnicity: string;
    nationality: string;
    career_start_year: number | null;
    career_end_year: number | null;
    height_cm: number | null;
    weight_kg: number | null;
    measurements: string;
    cupsize: string;
    hair_color: string;
    eye_color: string;
    tattoos: string;
    piercings: string;
    fake_boobs: boolean;
    same_sex_only: boolean;
};

defineProps<{
    form: FormState;
    isEditMode: boolean;
    loading: boolean;
    error: string;
    imagePreview: string | null;
    newAlias: string;
    genderOptions: { value: string; label: string }[];
}>();

const emit = defineEmits<{
    submit: [];
    close: [];
    'fetch-porndb': [];
    'image-change': [event: Event];
    'add-alias': [];
    'remove-alias': [index: number];
    'update:newAlias': [value: string];
}>();
</script>

<template>
    <div>
        <!-- Header -->
        <div class="mb-4 flex items-center justify-between">
            <h3 class="text-sm font-semibold text-white">
                {{ isEditMode ? 'Edit Actor' : 'Create Actor' }}
            </h3>
            <button
                type="button"
                class="border-border bg-panel hover:border-lava/50 hover:text-lava text-dim flex
                    items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-[11px]
                    transition-colors"
                @click="emit('fetch-porndb')"
            >
                <Icon name="heroicons:cloud-arrow-down" size="14" />
                Fetch from PornDB
            </button>
        </div>

        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <form class="space-y-4" @submit.prevent="emit('submit')">
            <!-- Image upload -->
            <div class="flex items-start gap-4">
                <div
                    class="bg-surface border-border relative h-32 w-24 shrink-0 overflow-hidden
                        rounded-lg border"
                >
                    <img
                        v-if="imagePreview"
                        :src="imagePreview"
                        class="h-full w-full object-cover"
                        alt="Actor preview"
                    />
                    <div v-else class="text-dim flex h-full w-full items-center justify-center">
                        <Icon name="heroicons:user" size="32" />
                    </div>
                </div>
                <div class="flex-1">
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Image
                    </label>
                    <input
                        type="file"
                        accept="image/*"
                        class="border-border bg-void/80 file:bg-panel file:text-dim w-full
                            rounded-lg border px-3 py-2 text-sm text-white file:mr-3 file:rounded-lg
                            file:border-0 file:px-3 file:py-1 file:text-xs"
                        @change="emit('image-change', $event)"
                    />
                    <div class="mt-2">
                        <label
                            class="text-dim mb-1 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Or Image URL
                        </label>
                        <input
                            v-model="form.image_url"
                            type="url"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none"
                            placeholder="https://..."
                        />
                    </div>
                </div>
            </div>

            <!-- Basic Info -->
            <div class="grid grid-cols-2 gap-3">
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Name *
                    </label>
                    <input
                        v-model="form.name"
                        type="text"
                        required
                        class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                            focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm text-white
                            transition-all focus:ring-1 focus:outline-none"
                    />
                </div>
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Gender
                    </label>
                    <select
                        v-model="form.gender"
                        class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                            w-full rounded-lg border px-3 py-2 text-sm text-white transition-all
                            focus:ring-1 focus:outline-none"
                    >
                        <option v-for="opt in genderOptions" :key="opt.value" :value="opt.value">
                            {{ opt.label }}
                        </option>
                    </select>
                </div>
            </div>

            <!-- Aliases -->
            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                >
                    Aliases
                </label>
                <div class="flex gap-2">
                    <input
                        :value="newAlias"
                        type="text"
                        placeholder="Add alias..."
                        class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                            focus:ring-lava/20 flex-1 rounded-lg border px-3 py-2 text-sm text-white
                            transition-all focus:ring-1 focus:outline-none"
                        @input="emit('update:newAlias', ($event.target as HTMLInputElement).value)"
                        @keyup.enter="emit('add-alias')"
                    />
                    <button
                        type="button"
                        :disabled="!newAlias.trim()"
                        class="border-border bg-panel hover:border-lava/50 hover:text-lava text-dim
                            shrink-0 rounded-lg border px-3 py-2 text-sm transition-colors
                            disabled:cursor-not-allowed disabled:opacity-40"
                        @click="emit('add-alias')"
                    >
                        Add
                    </button>
                </div>
                <div v-if="form.aliases.length > 0" class="mt-2 flex flex-wrap gap-1.5">
                    <span
                        v-for="(alias, index) in form.aliases"
                        :key="index"
                        class="border-border bg-surface inline-flex items-center gap-1 rounded-full
                            border px-2.5 py-1 text-xs text-white"
                    >
                        {{ alias }}
                        <button
                            type="button"
                            class="text-dim hover:text-lava transition-colors"
                            @click="emit('remove-alias', index)"
                        >
                            <Icon name="heroicons:x-mark" size="12" />
                        </button>
                    </span>
                </div>
            </div>

            <!-- Demographics -->
            <div class="grid grid-cols-2 gap-3">
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Birthday
                    </label>
                    <input
                        v-model="form.birthday"
                        type="date"
                        class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                            w-full rounded-lg border px-3 py-2 text-sm text-white transition-all
                            focus:ring-1 focus:outline-none"
                    />
                </div>
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Nationality
                    </label>
                    <input
                        v-model="form.nationality"
                        type="text"
                        class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                            focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm text-white
                            transition-all focus:ring-1 focus:outline-none"
                    />
                </div>
            </div>

            <div class="grid grid-cols-2 gap-3">
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Birthplace
                    </label>
                    <input
                        v-model="form.birthplace"
                        type="text"
                        class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                            focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm text-white
                            transition-all focus:ring-1 focus:outline-none"
                    />
                </div>
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Ethnicity
                    </label>
                    <input
                        v-model="form.ethnicity"
                        type="text"
                        class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                            focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm text-white
                            transition-all focus:ring-1 focus:outline-none"
                    />
                </div>
            </div>

            <!-- Physical -->
            <div class="grid grid-cols-4 gap-3">
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Height (cm)
                    </label>
                    <input
                        v-model.number="form.height_cm"
                        type="number"
                        min="0"
                        max="300"
                        class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                            w-full rounded-lg border px-3 py-2 text-sm text-white transition-all
                            focus:ring-1 focus:outline-none"
                    />
                </div>
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Weight (kg)
                    </label>
                    <input
                        v-model.number="form.weight_kg"
                        type="number"
                        min="0"
                        max="300"
                        class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                            w-full rounded-lg border px-3 py-2 text-sm text-white transition-all
                            focus:ring-1 focus:outline-none"
                    />
                </div>
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Measurements
                    </label>
                    <input
                        v-model="form.measurements"
                        type="text"
                        placeholder="34-24-34"
                        class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                            focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm text-white
                            transition-all focus:ring-1 focus:outline-none"
                    />
                </div>
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Cup Size
                    </label>
                    <input
                        v-model="form.cupsize"
                        type="text"
                        class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                            focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm text-white
                            transition-all focus:ring-1 focus:outline-none"
                    />
                </div>
            </div>

            <div class="grid grid-cols-2 gap-3">
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Hair Color
                    </label>
                    <input
                        v-model="form.hair_color"
                        type="text"
                        class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                            focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm text-white
                            transition-all focus:ring-1 focus:outline-none"
                    />
                </div>
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Eye Color
                    </label>
                    <input
                        v-model="form.eye_color"
                        type="text"
                        class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                            focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm text-white
                            transition-all focus:ring-1 focus:outline-none"
                    />
                </div>
            </div>

            <!-- Career -->
            <div class="grid grid-cols-2 gap-3">
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Career Start Year
                    </label>
                    <input
                        v-model.number="form.career_start_year"
                        type="number"
                        min="1900"
                        :max="new Date().getFullYear()"
                        class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                            w-full rounded-lg border px-3 py-2 text-sm text-white transition-all
                            focus:ring-1 focus:outline-none"
                    />
                </div>
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Career End Year
                    </label>
                    <input
                        v-model.number="form.career_end_year"
                        type="number"
                        min="1900"
                        :max="new Date().getFullYear()"
                        class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                            w-full rounded-lg border px-3 py-2 text-sm text-white transition-all
                            focus:ring-1 focus:outline-none"
                    />
                </div>
            </div>

            <!-- Body Modifications -->
            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                >
                    Tattoos
                </label>
                <textarea
                    v-model="form.tattoos"
                    rows="2"
                    class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                        focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm text-white
                        transition-all focus:ring-1 focus:outline-none"
                ></textarea>
            </div>

            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                >
                    Piercings
                </label>
                <textarea
                    v-model="form.piercings"
                    rows="2"
                    class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                        focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm text-white
                        transition-all focus:ring-1 focus:outline-none"
                ></textarea>
            </div>

            <!-- Toggles -->
            <div class="flex gap-6">
                <label class="flex items-center gap-2 text-sm text-white">
                    <input
                        v-model="form.fake_boobs"
                        type="checkbox"
                        class="accent-lava h-4 w-4 rounded"
                    />
                    Enhanced
                </label>
                <label class="flex items-center gap-2 text-sm text-white">
                    <input
                        v-model="form.same_sex_only"
                        type="checkbox"
                        class="accent-lava h-4 w-4 rounded"
                    />
                    Same-sex Only
                </label>
            </div>

            <!-- Actions -->
            <div class="flex justify-end gap-2 pt-2">
                <button
                    type="button"
                    class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                        hover:text-white"
                    @click="emit('close')"
                >
                    Cancel
                </button>
                <button
                    type="submit"
                    :disabled="loading || !form.name"
                    class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs font-semibold
                        text-white transition-all disabled:cursor-not-allowed disabled:opacity-40"
                >
                    {{ loading ? 'Saving...' : isEditMode ? 'Save Changes' : 'Create Actor' }}
                </button>
            </div>
        </form>
    </div>
</template>
