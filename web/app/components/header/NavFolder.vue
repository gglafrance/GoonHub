<script setup lang="ts">
import type { NavFolder } from '~/types/navLayout';
import type { NavItemDef } from '~/utils/navRegistry';

// A folder of the header layout (docs/NAVIGATION.md), shown as a dropdown button; the default
// layout has one, "Manage" (curation and management pages). Opens on click (touch too), under
// the button and extending to its right; closes on outside click, Esc, Tab out, item click and
// route change. Arrow keys, Home and End move between the items. A dot on the button tells
// that an item inside has a badge (unread messages, friend requests, public lobbies).
const props = defineProps<{ folder: NavFolder; items: NavItemDef[] }>();

const { badge, route: routeFor } = useHeaderItems();
const route = useRoute();
const open = ref(false);
const buttonRef = ref<HTMLButtonElement | null>(null);
const panelRef = ref<HTMLDivElement | null>(null);
const position = ref({ top: 0, left: 0 });
const menuId = useId();

const PANEL_WIDTH = 192;

const icon = computed(() => props.folder.icon || 'heroicons:folder');
const active = computed(() =>
    isAnyNavItemActive(
        route.path,
        props.items.map((i) => ({ to: routeFor(i) ?? undefined })),
    ),
);
const hasBadge = computed(() => props.items.some((i) => badge(i.id) !== null));

function itemEls(): HTMLElement[] {
    return Array.from(panelRef.value?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? []);
}

function focusItem(index: number) {
    itemEls()[index]?.focus();
}

function updatePosition() {
    if (!buttonRef.value) return;
    const rect = buttonRef.value.getBoundingClientRect();
    position.value = {
        top: rect.bottom + 6,
        left: menuPanelLeft(rect.left, PANEL_WIDTH, window.innerWidth),
    };
}

async function show(focus: 'first' | 'last' | null = null) {
    updatePosition();
    open.value = true;
    if (focus) {
        await nextTick();
        focusItem(focus === 'first' ? 0 : props.items.length - 1);
    }
}

function close(returnFocus = false) {
    if (!open.value) return;
    open.value = false;
    if (returnFocus) buttonRef.value?.focus();
}

function onButtonClick(e: MouseEvent) {
    if (open.value) {
        close();
        return;
    }
    // detail 0: activated with the keyboard (Enter/Space), focus goes into the menu
    show(e.detail === 0 ? 'first' : null);
}

function onButtonKeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault();
        show(e.key === 'ArrowDown' ? 'first' : 'last');
    }
}

function onPanelKeydown(e: KeyboardEvent) {
    if (e.key === 'Tab') {
        // the panel lives in <body>: hand the focus back to the button, the Tab goes on from it
        close(true);
        return;
    }
    const els = itemEls();
    const current = els.indexOf(document.activeElement as HTMLElement);
    const next = menuFocusIndex(current, els.length, e.key);
    if (next !== null) {
        e.preventDefault();
        els[next]?.focus();
    }
}

function onPointerDownOutside(e: PointerEvent) {
    const target = e.target as Node;
    if (panelRef.value?.contains(target) || buttonRef.value?.contains(target)) return;
    close();
}

function onDocumentKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
        e.stopPropagation();
        close(true);
    }
}

function teardown() {
    window.removeEventListener('resize', updatePosition);
    document.removeEventListener('pointerdown', onPointerDownOutside, true);
    document.removeEventListener('keydown', onDocumentKeydown, true);
}

watch(open, (isOpen) => {
    if (isOpen) {
        window.addEventListener('resize', updatePosition);
        document.addEventListener('pointerdown', onPointerDownOutside, true);
        document.addEventListener('keydown', onDocumentKeydown, true);
    } else {
        teardown();
    }
});

watch(
    () => route.fullPath,
    () => close(),
);

onBeforeUnmount(teardown);
</script>

<template>
    <div class="relative">
        <button
            ref="buttonRef"
            type="button"
            class="text-dim relative flex shrink-0 items-center justify-center gap-0.5 rounded-md
                px-2 py-1.5 text-[11px] font-medium transition-all hover:bg-white/5 hover:text-white
                sm:justify-start sm:gap-1.5 sm:px-2.5 sm:py-1"
            :class="{ '!text-lava bg-lava/10': active, 'bg-white/5 text-white': open && !active }"
            :title="folder.name"
            :aria-label="folder.name"
            aria-haspopup="menu"
            :aria-expanded="open"
            :aria-controls="open ? menuId : undefined"
            @click="onButtonClick"
            @keydown="onButtonKeydown"
        >
            <Icon :name="icon" size="18" class="sm:!h-3.5 sm:!w-3.5" />
            <span class="hidden sm:inline">{{ folder.name }}</span>
            <Icon
                name="heroicons:chevron-down"
                size="12"
                class="transition-transform duration-150"
                :class="{ 'rotate-180': open }"
            />
            <span
                v-if="hasBadge"
                class="bg-lava absolute top-0.5 right-0.5 h-1.5 w-1.5 rounded-full
                    shadow-[0_0_6px_rgba(255,77,77,0.6)]"
                aria-hidden="true"
            />
        </button>

        <Teleport to="body">
            <Transition
                enter-active-class="transition duration-100 ease-out"
                enter-from-class="-translate-y-1 opacity-0"
                leave-active-class="transition duration-75 ease-in"
                leave-to-class="-translate-y-1 opacity-0"
            >
                <div
                    v-if="open"
                    :id="menuId"
                    ref="panelRef"
                    role="menu"
                    :aria-label="folder.name"
                    class="border-border bg-panel/95 fixed z-[60] rounded-lg border p-1 shadow-2xl
                        backdrop-blur-md"
                    :style="{
                        top: `${position.top}px`,
                        left: `${position.left}px`,
                        width: `${PANEL_WIDTH}px`,
                    }"
                    @keydown="onPanelKeydown"
                >
                    <div
                        class="px-2.5 pt-1.5 pb-1 text-[10px] font-medium tracking-wider
                            text-white/40 uppercase"
                    >
                        {{ folder.name }}
                    </div>
                    <HeaderFolderItem
                        v-for="item in items"
                        :key="item.id"
                        :item="item"
                        @select="close()"
                    />
                </div>
            </Transition>
        </Teleport>
    </div>
</template>
