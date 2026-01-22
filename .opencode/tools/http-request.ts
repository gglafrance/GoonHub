import { tool } from "@opencode-ai/plugin";

async function runCommand(
    cmd: string,
    cwd?: string,
): Promise<{ stdout: string; stderr: string; exitCode: number }> {
    const proc = Bun.spawn({
        cmd: ["sh", "-c", cmd],
        cwd: cwd || process.cwd(),
        stdout: "pipe",
        stderr: "pipe",
    });

    const [stdout, stderr] = await Promise.all([
        new Response(proc.stdout).text(),
        new Response(proc.stderr).text(),
    ]);

    const exitCode = await proc.exited;
    return { stdout, stderr, exitCode };
}

export default tool({
    description:
        "Make HTTP requests with automatic JSON parsing and structured responses",
    args: {
        method: tool.schema
            .string()
            .describe(
                "HTTP method: GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD",
            ),
        url: tool.schema.string().describe("Full URL to request"),
        headers: tool.schema
            .string()
            .optional()
            .describe("Additional headers as JSON string of key-value pairs"),
        body: tool.schema
            .string()
            .optional()
            .describe("JSON request body as string"),
        bearerToken: tool.schema
            .string()
            .optional()
            .describe("Bearer token for authorization"),
        timeout: tool.schema
            .number()
            .optional()
            .describe("Request timeout in seconds (default: 30)"),
        outputHeaders: tool.schema
            .boolean()
            .optional()
            .describe("Include response headers in output"),
        expectJson: tool.schema
            .boolean()
            .optional()
            .describe("Parse response as JSON (default: true)"),
    },
    async execute(args, context) {
        const {
            method,
            url,
            headers,
            body,
            bearerToken,
            timeout = 30,
            outputHeaders = false,
            expectJson = true,
        } = args;

        try {
            let curlCmd = `curl -s -w "\\nHTTP_STATUS:%{http_code}\\nHTTP_HEADERS:%{num_headers}" -X ${method} "${url}"`;

            if (headers) {
                const headersObj = JSON.parse(headers);
                Object.entries(headersObj).forEach(([key, value]) => {
                    curlCmd += ` -H "${key}: ${value}"`;
                });
            }

            if (bearerToken) {
                curlCmd += ` -H "Authorization: Bearer ${bearerToken}"`;
            }

            if (body && ["POST", "PUT", "PATCH"].includes(method)) {
                curlCmd += ` -H "Content-Type: application/json" -d '${body}'`;
            }

            curlCmd += ` --max-time ${timeout}`;

            const result = await runCommand(curlCmd);

            const lines = result.stdout.split("\n");
            let statusCode = 0;
            let numHeaders = 0;
            let responseBody = "";
            let responseHeaders: Record<string, string> = {};

            for (let i = 0; i < lines.length; i++) {
                const line = lines[i];

                if (line.startsWith("HTTP_STATUS:")) {
                    statusCode = parseInt(
                        line.replace("HTTP_STATUS:", "").trim(),
                    );
                } else if (line.startsWith("HTTP_HEADERS:")) {
                    numHeaders = parseInt(
                        line.replace("HTTP_HEADERS:", "").trim(),
                    );
                } else if (i < numHeaders && line.includes(":")) {
                    const [key, ...valueParts] = line.split(":");
                    const value = valueParts.join(":").trim();
                    if (key && value) {
                        responseHeaders[key.trim()] = value;
                    }
                } else {
                    responseBody += line + "\n";
                }
            }

            responseBody = responseBody.trim();

            let data: any = responseBody;
            if (expectJson && responseBody.length > 0) {
                try {
                    data = JSON.parse(responseBody);
                } catch {
                    data = responseBody;
                }
            } else if (responseBody.length === 0) {
                data = null;
            }

            const results: string[] = [];
            results.push(`Method: ${method}`);
            results.push(`URL: ${url}`);
            results.push(`Status: ${statusCode}`);
            results.push(`Success: ${statusCode >= 200 && statusCode < 300}`);

            if (outputHeaders) {
                results.push(`Headers: ${JSON.stringify(responseHeaders)}`);
            }

            results.push(
                `Data: ${typeof data === "object" ? JSON.stringify(data) : data}`,
            );
            return results.join("\n");
        } catch (error: any) {
            return `Error: ${error.message}`;
        }
    },
});
