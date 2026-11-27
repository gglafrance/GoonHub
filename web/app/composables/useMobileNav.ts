/**
 * Shared state of the mobile navigation (docs/NAVIGATION.md, "Mobile"): whether the viewport is
 * below the md breakpoint, whether the Menu sheet is open, and whether a text field has the
 * focus (the on-screen keyboard is up: the bottom tab bar steps aside).
 *
 * The layout height of the chrome is exposed as CSS variables on <html> so that pages and
 * fixed elements can stay clear of it: --app-header-h (the sticky header) and
 * --app-bottom-nav-h (the bottom tab bar, 0px when it is not shown).
 */
export function useMobileNav() {
    const isMobile = useState('mobile-nav:is-mobile', () => false);
    const menuOpen = useState('mobile-nav:menu-open', () => false);
    const typing = useState('mobile-nav:typing', () => false);

    function openMenu() {
        menuOpen.value = true;
    }
    function closeMenu() {
        menuOpen.value = false;
    }
    function toggleMenu() {
        menuOpen.value = !menuOpen.value;
    }

    return { isMobile, menuOpen, typing, openMenu, closeMenu, toggleMenu };
}

const TEXT_INPUT_TYPES = new Set([
    '',
    'text',
    'search',
    'email',
    'url',
    'tel',
    'password',
    'number',
    'date',
    'datetime-local',
    'time',
]);

/** Whether an element brings up the on-screen keyboard. */
function isTextField(el: Element | null): boolean {
    if (!el) return false;
    if (el instanceof HTMLTextAreaElement) return !el.readOnly;
    if (el instanceof HTMLInputElement) return !el.readOnly && TEXT_INPUT_TYPES.has(el.type);
    return el instanceof HTMLElement && el.isContentEditable;
}

/**
 * Starts the listeners behind useMobileNav (once, from AppHeader): the media query and the
 * focus of text fields. Returns a stop function.
 */
export function startMobileNavTracking(): () => void {
    const { isMobile, menuOpen, typing } = useMobileNav();
    const mql = window.matchMedia(MOBILE_NAV_QUERY);
    const onChange = () => {
        isMobile.value = mql.matches;
        if (!mql.matches) menuOpen.value = false;
    };
    onChange();
    mql.addEventListener('change', onChange);

    const onFocus = () => {
        typing.value = isTextField(document.activeElement);
    };
    // focusout fires before the next element gets the focus: read it on the next frame
    const onBlur = () => requestAnimationFrame(onFocus);
    document.addEventListener('focusin', onFocus);
    document.addEventListener('focusout', onBlur);

    return () => {
        mql.removeEventListener('change', onChange);
        document.removeEventListener('focusin', onFocus);
        document.removeEventListener('focusout', onBlur);
    };
}

/** Sets a CSS length variable on <html> (removed with null: the stylesheet default applies). */
export function setChromeVar(name: '--app-header-h' | '--app-bottom-nav-h', value: string | null) {
    const style = document.documentElement.style;
    if (value === null) style.removeProperty(name);
    else style.setProperty(name, value);
}
