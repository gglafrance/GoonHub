import type { HomepageResponse, HomepageSectionData } from '~/types/homepage';

/**
 * Homepage API operations: fetching homepage data and section data.
 */
export const useApiHomepage = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const fetchHomepageData = async (): Promise<HomepageResponse> => {
        const response = await fetch('/api/v1/homepage', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchSectionData = async (sectionId: string): Promise<HomepageSectionData> => {
        const response = await fetch(`/api/v1/homepage/sections/${sectionId}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    return {
        fetchHomepageData,
        fetchSectionData,
    };
};
