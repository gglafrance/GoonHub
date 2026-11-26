// Copy to the clipboard, also where navigator.clipboard is missing: the app is often opened
// over plain http on a LAN address, which is not a secure context (no Clipboard API there).
// Used through useClipboard(); tests in web/tests/clipboard.test.ts.

export interface ClipboardEnv {
    /** navigator.clipboard (undefined outside secure contexts) */
    clipboard?: { writeText: (text: string) => Promise<void> } | null;
    /** window.isSecureContext */
    secure?: boolean;
    document?: Document | null;
}

const browserEnv = (): ClipboardEnv =>
    typeof window === 'undefined'
        ? {}
        : {
              clipboard: navigator.clipboard ?? null,
              secure: window.isSecureContext,
              document,
          };

/**
 * Copies with the legacy `execCommand('copy')` from a hidden, selected textarea (works in
 * non-secure contexts, during a user gesture). The page selection is restored afterwards.
 */
export function legacyCopy(text: string, doc: Document): boolean {
    if (!doc.body) return false;
    const area = doc.createElement('textarea');
    area.value = text;
    area.setAttribute('readonly', '');
    area.setAttribute('aria-hidden', 'true');
    Object.assign(area.style, {
        position: 'fixed',
        top: '0',
        left: '0',
        width: '1px',
        height: '1px',
        opacity: '0',
        pointerEvents: 'none',
    });
    const selection = doc.getSelection?.() ?? null;
    const previous = selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null;
    const focused = doc.activeElement as HTMLElement | null;
    doc.body.appendChild(area);
    let ok = false;
    try {
        area.focus();
        area.select();
        area.setSelectionRange(0, text.length);
        ok = doc.execCommand('copy');
    } catch {
        ok = false;
    } finally {
        doc.body.removeChild(area);
        if (previous && selection) {
            selection.removeAllRanges();
            selection.addRange(previous);
        }
        focused?.focus?.();
    }
    return ok;
}

/**
 * Copies text: the Clipboard API when available (secure context), else the legacy fallback.
 * Resolves to false when both fail (show the text so the user can copy it by hand).
 */
export async function copyText(text: string, env: ClipboardEnv = browserEnv()): Promise<boolean> {
    if (env.clipboard && env.secure !== false) {
        try {
            await env.clipboard.writeText(text);
            return true;
        } catch {
            // Denied or unavailable: try the fallback
        }
    }
    return env.document ? legacyCopy(text, env.document) : false;
}
