/**
 * Reads the JSON body of an API response. A response without content (204 No Content, 205,
 * or an empty body: DELETE endpoints answer that way) gives null instead of throwing
 * "Unexpected end of JSON input".
 */
export async function readResponseBody(response: Response) {
    if (response.status === 204 || response.status === 205) return null;
    const text = await response.text();
    if (text.trim() === '') return null;
    return JSON.parse(text);
}
