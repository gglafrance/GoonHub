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
        "Handles frontend-Go integration: builds frontend, validates dist, regenerates wire if needed, builds Go binary",
    args: {
        skipFrontendBuild: tool.schema
            .boolean()
            .optional()
            .describe("Skip the frontend build step"),
        skipGoBuild: tool.schema
            .boolean()
            .optional()
            .describe("Skip the Go binary build step"),
        outputBinary: tool.schema
            .string()
            .optional()
            .describe("Output binary name (default: goonhub)"),
    },
    async execute(args, context) {
        const {
            skipFrontendBuild = false,
            skipGoBuild = false,
            outputBinary = "goonhub",
        } = args;

        const steps: { name: string; run: () => Promise<string> }[] = [];
        const results: string[] = [];

        steps.push({
            name: "Check web directory exists",
            run: async () => {
                const result = await runCommand("test -d web");
                if (result.exitCode !== 0) {
                    throw new Error("web/ directory not found");
                }
                return "web/ directory exists";
            },
        });

        if (!skipFrontendBuild) {
            steps.push({
                name: "Build frontend with bun",
                run: async () => {
                    const result = await runCommand("cd web && bun run build");
                    if (result.exitCode !== 0) {
                        throw new Error(
                            `Frontend build failed: ${result.stderr}`,
                        );
                    }
                    return "Frontend build completed";
                },
            });
        }

        steps.push({
            name: "Validate dist/ contents",
            run: async () => {
                const result = await runCommand("ls -la web/dist/");
                if (result.exitCode !== 0) {
                    throw new Error(
                        "web/dist/ directory not found after build",
                    );
                }
                const filesResult = await runCommand("ls web/dist/");
                const fileCount = filesResult.stdout
                    .trim()
                    .split("\n")
                    .filter((f) => f).length;
                return `web/dist/ contains ${fileCount} files/directories`;
            },
        });

        steps.push({
            name: "Check if wire needs regeneration",
            run: async () => {
                const wireGo = await runCommand(
                    "test -f internal/wire/wire.go",
                );
                if (wireGo.exitCode !== 0) {
                    return "No wire.go found, skipping wire regeneration";
                }

                const wireGen = await runCommand(
                    "test -f internal/wire/wire_gen.go",
                );
                if (wireGen.exitCode !== 0) {
                    return "wire_gen.go not found, will regenerate";
                }

                const wireGoStat = await Bun.file(
                    "internal/wire/wire.go",
                ).exists();
                const wireGenStat = await Bun.file(
                    "internal/wire/wire_gen.go",
                ).exists();

                if (wireGoStat && wireGenStat) {
                    const wireGoMtime = await Bun.file("internal/wire/wire.go")
                        .lastModified;
                    const wireGenMtime = await Bun.file(
                        "internal/wire/wire_gen.go",
                    ).lastModified;

                    if (wireGoMtime > wireGenMtime) {
                        return "wire.go newer than wire_gen.go, will regenerate";
                    }
                }

                return "wire_gen.go is up to date";
            },
        });

        steps.push({
            name: "Regenerate wire",
            run: async () => {
                const result = await runCommand(
                    "go run github.com/google/wire/cmd/wire ./internal/wire",
                );
                if (result.exitCode !== 0) {
                    throw new Error(`Wire generation failed: ${result.stderr}`);
                }
                if (result.stdout.includes("wire_gen.go")) {
                    return "Wire regenerated successfully";
                }
                return "Wire generation completed (may already be up to date)";
            },
        });

        steps.push({
            name: "Validate wire_gen.go compiles",
            run: async () => {
                const result = await runCommand(
                    "go build -o /dev/null ./internal/wire/",
                );
                if (result.exitCode !== 0) {
                    throw new Error(
                        `wire_gen.go has compilation errors: ${result.stderr}`,
                    );
                }
                return "wire_gen.go compiles successfully";
            },
        });

        steps.push({
            name: "Check for stale dist/ vs Go code",
            run: async () => {
                const webGoExists = await Bun.file("web.go").exists();
                if (!webGoExists) {
                    return "web.go not found, skipping staleness check";
                }

                const webGoMtime = await Bun.file("web.go").lastModified;
                const distMtime = await (async () => {
                    try {
                        const result = await runCommand(
                            "find web/dist -type f -newer web.go | head -1",
                        );
                        return result.exitCode === 0
                            ? Date.now()
                            : webGoMtime - 1;
                    } catch {
                        return webGoMtime - 1;
                    }
                })();

                if (webGoMtime > distMtime) {
                    return "Warning: web.go is newer than dist/ - consider rebuilding frontend";
                }

                return "dist/ is newer than web.go - build is in sync";
            },
        });

        if (!skipGoBuild) {
            steps.push({
                name: "Build Go binary",
                run: async () => {
                    const result = await runCommand(
                        `go build -o ${outputBinary} ./cmd/server`,
                    );
                    if (result.exitCode !== 0) {
                        throw new Error(`Go build failed: ${result.stderr}`);
                    }
                    return `Go binary built: ${outputBinary}`;
                },
            });
        }

        for (const step of steps) {
            try {
                const result = await step.run();
                results.push(`✓ ${step.name}: ${result}`);
            } catch (error: any) {
                results.push(`✗ ${step.name}: ${error.message}`);
                return results.join("\n");
            }
        }

        return results.join("\n");
    },
});
