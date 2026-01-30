import type {
    StoragePathsResponse,
    FolderContentsResponse,
    BulkUpdateTagsRequest,
    BulkUpdateActorsRequest,
    BulkUpdateStudioRequest,
    FolderVideoIDsRequest,
    BulkUpdateResponse,
    FolderVideoIDsResponse,
    BulkDeleteRequest,
    BulkDeleteResponse,
    FolderSearchRequest,
    FolderSearchResponse,
} from '~/types/explorer';

/**
 * Explorer-related API operations: folder browsing and bulk editing.
 */
export const useApiExplorer = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const getStoragePaths = async (): Promise<StoragePathsResponse> => {
        const response = await fetch('/api/v1/explorer/storage-paths', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getFolderContents = async (
        storagePathID: number,
        folderPath: string,
        page = 1,
        limit = 24,
    ): Promise<FolderContentsResponse> => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });

        // Ensure path starts with / for the API
        const normalizedPath = folderPath.startsWith('/') ? folderPath : `/${folderPath}`;

        const response = await fetch(
            `/api/v1/explorer/folders/${storagePathID}${normalizedPath}?${params}`,
            {
                headers: getAuthHeaders(),
                ...fetchOptions(),
            },
        );
        return handleResponse(response);
    };

    const bulkUpdateTags = async (request: BulkUpdateTagsRequest): Promise<BulkUpdateResponse> => {
        const response = await fetch('/api/v1/explorer/bulk/tags', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(request),
        });
        return handleResponse(response);
    };

    const bulkUpdateActors = async (
        request: BulkUpdateActorsRequest,
    ): Promise<BulkUpdateResponse> => {
        const response = await fetch('/api/v1/explorer/bulk/actors', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(request),
        });
        return handleResponse(response);
    };

    const bulkUpdateStudio = async (
        request: BulkUpdateStudioRequest,
    ): Promise<BulkUpdateResponse> => {
        const response = await fetch('/api/v1/explorer/bulk/studio', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(request),
        });
        return handleResponse(response);
    };

    const getFolderVideoIDs = async (
        request: FolderVideoIDsRequest,
    ): Promise<FolderVideoIDsResponse> => {
        const response = await fetch('/api/v1/explorer/folder/video-ids', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(request),
        });
        return handleResponse(response);
    };

    const bulkDeleteVideos = async (request: BulkDeleteRequest): Promise<BulkDeleteResponse> => {
        const response = await fetch('/api/v1/explorer/bulk/videos', {
            method: 'DELETE',
            headers: getAuthHeaders(),
            body: JSON.stringify(request),
        });
        return handleResponse(response);
    };

    const searchInFolder = async (request: FolderSearchRequest): Promise<FolderSearchResponse> => {
        const response = await fetch('/api/v1/explorer/search', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(request),
        });
        return handleResponse(response);
    };

    return {
        getStoragePaths,
        getFolderContents,
        bulkUpdateTags,
        bulkUpdateActors,
        bulkUpdateStudio,
        getFolderVideoIDs,
        bulkDeleteVideos,
        searchInFolder,
    };
};
