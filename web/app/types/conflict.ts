// 409 answer of a strict actor/studio create or update: the name or an alias is already
// used by another entity (`existing`).
export interface EntityConflict<T> {
    error: string;
    code: string;
    field: 'name' | 'aliases';
    value: string;
    existing: T;
}
