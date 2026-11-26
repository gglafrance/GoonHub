import { describe, expect, test } from 'bun:test';
import { copyText, legacyCopy } from '../app/utils/clipboard';

// A minimal document for the execCommand fallback (no DOM in bun test)
function fakeDocument(execResult: boolean | 'throw') {
    const calls = { appended: 0, removed: 0, copied: '' as string, commands: [] as string[] };
    let current: { value: string } | null = null;
    const doc = {
        body: {
            appendChild: (el: { value: string }) => {
                calls.appended++;
                current = el;
            },
            removeChild: () => {
                calls.removed++;
            },
        },
        activeElement: null,
        getSelection: () => null,
        createElement: () => ({
            value: '',
            style: {},
            setAttribute: () => {},
            focus: () => {},
            select: () => {},
            setSelectionRange: () => {},
        }),
        execCommand: (cmd: string) => {
            calls.commands.push(cmd);
            if (execResult === 'throw') throw new Error('not allowed');
            calls.copied = current?.value ?? '';
            return execResult;
        },
    };
    return { doc: doc as unknown as Document, calls };
}

describe('copyText', () => {
    test('uses the Clipboard API in a secure context', async () => {
        let written = '';
        const { doc, calls } = fakeDocument(true);
        const ok = await copyText('hello', {
            clipboard: { writeText: async (t) => void (written = t) },
            secure: true,
            document: doc,
        });
        expect(ok).toBe(true);
        expect(written).toBe('hello');
        expect(calls.commands).toEqual([]);
    });

    test('falls back to execCommand without the Clipboard API (plain http)', async () => {
        const { doc, calls } = fakeDocument(true);
        const ok = await copyText('http://192.168.1.5/together/join/ABC', {
            clipboard: null,
            secure: false,
            document: doc,
        });
        expect(ok).toBe(true);
        expect(calls.copied).toBe('http://192.168.1.5/together/join/ABC');
        expect(calls.appended).toBe(1);
        expect(calls.removed).toBe(1);
    });

    test('falls back when the Clipboard API refuses', async () => {
        const { doc, calls } = fakeDocument(true);
        const ok = await copyText('x', {
            clipboard: {
                writeText: async () => {
                    throw new Error('denied');
                },
            },
            secure: true,
            document: doc,
        });
        expect(ok).toBe(true);
        expect(calls.commands).toEqual(['copy']);
    });

    test('resolves false when everything fails (the text is then shown)', async () => {
        const refused = fakeDocument(false);
        expect(await copyText('x', { clipboard: null, document: refused.doc })).toBe(false);
        const throwing = fakeDocument('throw');
        expect(legacyCopy('x', throwing.doc)).toBe(false);
        expect(throwing.calls.removed).toBe(1);
        expect(await copyText('x', {})).toBe(false);
    });
});
