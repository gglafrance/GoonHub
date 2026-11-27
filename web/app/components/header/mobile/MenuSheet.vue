<script setup lang="ts">
import type { MobileNav } from '~/utils/mobileNav';

// Mobile Menu sheet (docs/NAVIGATION.md, "Mobile"): slides up above the tab bar with
// everything the tab bar does not hold: the profile, the other pages (one grid per folder of
// the layout), the social rows, the quick toggles and the account rows. Closes on the Menu tab,
// the backdrop, Esc, a swipe down on its handle and any navigation.
const props = defineProps<{ nav: MobileNav }>();

const route = useRoute();
const authStore = useAuthStore();
const perms = useMyPermissions();
const socialStore = useSocialStore();
const { menuOpen, closeMenu } = useMobileNav();

const NuxtLink = resolveComponent('NuxtLink');
const sheetRef = ref<HTMLElement | null>(null);
const dragY = ref(0);
let dragStart: number | null = null;

const user = computed(() => authStore.user);
const displayName = computed(() => socialStore.me?.display_name || user.value?.username || '');
const hasRows = computed(
    () => props.nav.social.length + props.nav.toggles.length + props.nav.account.length > 0,
);

watch(
    () => route.fullPath,
    () => closeMenu(),
);

watch(menuOpen, async (open) => {
    document.body.style.overflow = open ? 'hidden' : '';
    dragY.value = 0;
    if (open) {
        await nextTick();
        sheetRef.value?.focus({ preventScroll: true });
    }
});

function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && menuOpen.value) {
        e.stopPropagation();
        closeMenu();
    }
}
onMounted(() => document.addEventListener('keydown', onKeydown));
onBeforeUnmount(() => {
    document.removeEventListener('keydown', onKeydown);
    document.body.style.overflow = '';
    closeMenu();
});

// Swipe down on the handle to close
function onTouchStart(e: TouchEvent) {
    dragStart = e.touches[0]?.clientY ?? null;
}
function onTouchMove(e: TouchEvent) {
    if (dragStart === null) return;
    dragY.value = Math.max(0, (e.touches[0]?.clientY ?? dragStart) - dragStart);
}
function onTouchEnd() {
    if (dragY.value > 80) closeMenu();
    dragY.value = 0;
    dragStart = null;
}
</script>

<template>
    <Teleport to="body">
        <Transition
            enter-active-class="transition-opacity duration-200"
            enter-from-class="opacity-0"
            leave-active-class="transition-opacity duration-150"
            leave-to-class="opacity-0"
        >
            <div
                v-if="menuOpen"
                class="fixed inset-x-0 top-0 bottom-[var(--app-bottom-nav-h)] z-[44] bg-black/60
                    backdrop-blur-sm"
                aria-hidden="true"
                @click="closeMenu()"
            />
        </Transition>

        <Transition
            enter-active-class="transition duration-200 ease-out"
            enter-from-class="translate-y-full"
            leave-active-class="transition duration-150 ease-in"
            leave-to-class="translate-y-full"
        >
            <div
                v-if="menuOpen"
                id="mobile-menu-sheet"
                ref="sheetRef"
                role="dialog"
                aria-modal="true"
                aria-label="Menu"
                tabindex="-1"
                class="border-border bg-surface fixed inset-x-0 bottom-[var(--app-bottom-nav-h)]
                    z-[44] flex
                    max-h-[calc(100dvh-var(--app-header-h)-var(--app-bottom-nav-h)-0.5rem)] flex-col
                    rounded-t-2xl border-t shadow-[0_-12px_40px_rgba(0,0,0,0.6)] outline-none"
                :style="dragY ? { transform: `translateY(${dragY}px)`, transition: 'none' } : {}"
            >
                <!-- Handle + profile (drag area) -->
                <div
                    class="shrink-0 touch-none"
                    @touchstart.passive="onTouchStart"
                    @touchmove.passive="onTouchMove"
                    @touchend="onTouchEnd"
                    @touchcancel="onTouchEnd"
                >
                    <div class="flex justify-center pt-2.5 pb-1.5">
                        <div class="h-1 w-10 rounded-full bg-white/20" />
                    </div>
                    <div v-if="user" class="flex items-center gap-3 px-4 pt-1 pb-3">
                        <component
                            :is="nav.profile ? NuxtLink : 'div'"
                            :to="nav.profile ? profilePath(user.username) : undefined"
                            class="flex min-w-0 flex-1 items-center gap-3"
                        >
                            <SocialUserAvatar
                                :username="user.username"
                                :display-name="socialStore.me?.display_name"
                                :user-id="socialStore.me?.id"
                                :avatar-version="socialStore.me?.avatar_version"
                                size="sm"
                                class="!h-10 !w-10 shrink-0"
                            />
                            <div class="min-w-0">
                                <div class="truncate text-sm font-semibold text-white">
                                    {{ displayName }}
                                </div>
                                <div class="flex items-center gap-1.5 text-[11px]">
                                    <span class="text-dim truncate">@{{ user.username }}</span>
                                    <span
                                        class="font-mono text-[10px] tracking-wider uppercase"
                                        :class="
                                            perms.isSuperuser.value ? 'text-emerald' : 'text-dim'
                                        "
                                    >
                                        {{ user.role }}
                                    </span>
                                </div>
                            </div>
                            <span
                                v-if="nav.profile"
                                class="text-dim ml-auto flex shrink-0 items-center gap-0.5
                                    text-[11px]"
                            >
                                Profile
                                <Icon name="heroicons:chevron-right" size="14" />
                            </span>
                        </component>
                    </div>
                </div>

                <!-- Content -->
                <div
                    class="border-border min-h-0 flex-1 overflow-y-auto overscroll-contain border-t
                        pb-2"
                >
                    <section v-for="section in nav.sections" :key="section.key" class="px-4 pt-3">
                        <h2
                            class="mb-2 flex items-center gap-1.5 text-[10px] font-semibold
                                tracking-wider text-white/40 uppercase"
                        >
                            <Icon :name="section.icon" size="12" />
                            {{ section.title }}
                        </h2>
                        <div class="grid grid-cols-3 gap-2 min-[400px]:grid-cols-4">
                            <HeaderMobileMenuTile
                                v-for="item in section.items"
                                :key="item.id"
                                :item="item"
                            />
                        </div>
                    </section>

                    <div v-if="hasRows" class="mt-3 space-y-3">
                        <div
                            v-for="(rows, i) in [nav.social, nav.toggles, nav.account]"
                            v-show="rows.length > 0"
                            :key="i"
                            class="border-border bg-panel/60 mx-4 divide-y divide-white/5
                                overflow-hidden rounded-xl border"
                        >
                            <HeaderMobileMenuRow v-for="item in rows" :key="item.id" :item="item" />
                        </div>
                    </div>
                </div>
            </div>
        </Transition>
    </Teleport>
</template>
