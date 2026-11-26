/**
 * Server-wide marker labels (Settings > Marker Labels, marker_labels:manage). Everyone lists
 * the labels through useApiMarkers().fetchLabelSuggestions (GET /markers/labels).
 */
import type {
    MarkerLabel,
    MarkerLabelDeleteResult,
    MarkerLabelInput,
    MarkerLabelMergeResult,
    MarkerLabelWithUsage,
} from '~/types/markerLabel';
import type { Tag } from '~/types/tag';

export const useApiMarkerLabels = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const send = async (method: string, path: string, body?: unknown) => {
        const response = await fetch(`/api/v1/marker-labels${path}`, {
            ...fetchOptions(),
            method,
            headers: getAuthHeaders(),
            body: body === undefined ? undefined : JSON.stringify(body),
        });
        return handleResponse(response);
    };

    const fetchMarkerLabels = async (): Promise<MarkerLabelWithUsage[]> => {
        const res: { data: MarkerLabelWithUsage[] } = await send('GET', '');
        return res.data || [];
    };

    /** Creates a label; a name used by another label answers 409 (see getEntityConflict). */
    const createMarkerLabel = (input: MarkerLabelInput): Promise<MarkerLabel> =>
        send('POST', '', input);

    /** Renames, recolors or describes a label; a name used by another label answers 409. */
    const updateMarkerLabel = (id: number, input: MarkerLabelInput): Promise<MarkerLabel> =>
        send('PUT', `/${id}`, input);

    /** Deletes labels: their markers stay, without label. */
    const deleteMarkerLabels = (ids: number[]): Promise<MarkerLabelDeleteResult> =>
        send('POST', '/delete', { ids });

    /** Merges sourceIds into targetId; with dryRun only counts what would change. */
    const mergeMarkerLabels = (
        targetId: number,
        sourceIds: number[],
        dryRun = false,
    ): Promise<MarkerLabelMergeResult> =>
        send('POST', '/merge', { target_id: targetId, source_ids: sourceIds, dry_run: dryRun });

    const setMarkerLabelTags = async (id: number, tagIds: number[]): Promise<Tag[]> => {
        const res: { tags: Tag[] } = await send('PUT', `/${id}/tags`, { tag_ids: tagIds });
        return res.tags || [];
    };

    return {
        fetchMarkerLabels,
        createMarkerLabel,
        updateMarkerLabel,
        deleteMarkerLabels,
        mergeMarkerLabels,
        setMarkerLabelTags,
    };
};
