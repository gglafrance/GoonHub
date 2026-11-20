// Minimal, safe markdown rendering for user text blocks: HTML is escaped first, then a small
// subset is converted (headings, bold, italic, inline code, links, lists, paragraphs).

const escapeHtml = (text: string): string =>
    text
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');

/** Only relative app links and http(s) URLs are allowed. */
export function isSafeLinkUrl(url: string): boolean {
    return /^\/(?!\/)/.test(url) || /^https?:\/\//i.test(url);
}

const inline = (text: string): string =>
    text
        .replace(
            /`([^`]+)`/g,
            '<code class="rounded bg-white/10 px-1 font-mono text-[0.9em]">$1</code>',
        )
        .replace(/\*\*([^*]+)\*\*/g, '<strong class="text-white">$1</strong>')
        .replace(/\*([^*]+)\*/g, '<em>$1</em>')
        .replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (_m, label: string, url: string) => {
            // url is already HTML-escaped; unescape the ampersands to test the scheme
            const raw = url.replace(/&amp;/g, '&');
            if (!isSafeLinkUrl(raw)) return label;
            const external = /^https?:/i.test(raw);
            return `<a href="${url}" class="text-lava hover:underline"${
                external ? ' target="_blank" rel="noopener noreferrer"' : ''
            }>${label}</a>`;
        });

export function renderMiniMarkdown(source: string): string {
    const lines = escapeHtml(source ?? '').split(/\r?\n/);
    const out: string[] = [];
    let list: string[] = [];
    let paragraph: string[] = [];

    const flushList = () => {
        if (list.length) {
            out.push(`<ul class="list-disc space-y-0.5 pl-5">${list.join('')}</ul>`);
            list = [];
        }
    };
    const flushParagraph = () => {
        if (paragraph.length) {
            out.push(`<p>${paragraph.join('<br>')}</p>`);
            paragraph = [];
        }
    };

    for (const line of lines) {
        const heading = line.match(/^(#{1,3})\s+(.*)$/);
        const item = line.match(/^\s*[-*]\s+(.*)$/);
        if (heading) {
            flushList();
            flushParagraph();
            const level = heading[1]!.length;
            const size = level === 1 ? 'text-base' : level === 2 ? 'text-sm' : 'text-xs';
            out.push(
                `<h${level + 2} class="${size} font-semibold text-white">${inline(heading[2]!)}</h${level + 2}>`,
            );
        } else if (item) {
            flushParagraph();
            list.push(`<li>${inline(item[1]!)}</li>`);
        } else if (line.trim() === '') {
            flushList();
            flushParagraph();
        } else {
            flushList();
            paragraph.push(inline(line));
        }
    }
    flushList();
    flushParagraph();
    return out.join('');
}
