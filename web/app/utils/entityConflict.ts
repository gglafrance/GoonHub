import type { EntityConflict } from '~/types/conflict';

/**
 * Returns the conflict carried by an API error (thrown by useApiCore.handleResponse) of a
 * strict actor/studio create or update, or null when the error is something else.
 */
export function getEntityConflict<T>(err: unknown): EntityConflict<T> | null {
    if (!(err instanceof Error)) return null;
    const { status, body } = err as Error & { status?: number; body?: Partial<EntityConflict<T>> };
    if (status !== 409 || !body || !body.existing || !body.field) return null;
    return body as EntityConflict<T>;
}
