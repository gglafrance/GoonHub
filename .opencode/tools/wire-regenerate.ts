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
        "Smart Wire generation: detects when wire providers need updating, runs wire gen, validates compilation, reports missing providers",
    args: {
        force: tool.schema
            .boolean()
            .optional()
            .describe("Force regenerate wire even if up to date"),
        verbose: tool.schema
            .boolean()
            .optional()
            .describe("Show detailed wire output"),
    },
    async execute(args, context) {
        const { force = false, verbose = false } = args;

        const results: string[] = [];

        const checkWireGo = async () => {
            const result = await runCommand("test -f internal/wire/wire.go");
            if (result.exitCode !== 0) {
                throw new Error(
                    "internal/wire/wire.go not found - cannot regenerate wire",
                );
            }
            return "wire.go found";
        };

        const checkStaleness = async () => {
            const wireGoMtime = await Bun.file("internal/wire/wire.go")
                .lastModified;
            const wireGenResult = await runCommand(
                "test -f internal/wire/wire_gen.go",
            );

            if (wireGenResult.exitCode !== 0) {
                return "wire_gen.go does not exist - needs generation";
            }

            const wireGenMtime = await Bun.file("internal/wire/wire_gen.go")
                .lastModified;

            if (wireGoMtime > wireGenMtime) {
                return `wire.go is newer (${wireGoMtime}) than wire_gen.go (${wireGenMtime}) - needs regeneration`;
            }

            return `wire_gen.go is up to date (last modified: ${wireGenMtime})`;
        };

        const runWire = async () => {
            const result = await runCommand(
                "go run github.com/google/wire/cmd/wire ./internal/wire",
            );
            if (result.exitCode !== 0) {
                throw new Error(`Wire generation failed: ${result.stderr}`);
            }

            if (verbose) {
                return (
                    result.stdout ||
                    result.stderr ||
                    "Wire generation completed"
                );
            }

            if (result.stdout.includes("wire_gen.go")) {
                return "Wire generation completed";
            }

            return "Wire generation completed (no changes needed)";
        };

        const validateCompilation = async () => {
            const result = await runCommand(
                "go build -o /dev/null ./internal/wire/",
            );
            if (result.exitCode !== 0) {
                const errorResult = await runCommand(
                    "go build ./internal/wire/ 2>&1",
                );
                throw new Error(
                    `wire_gen.go has compilation errors:\n${errorResult.stderr}`,
                );
            }
            return "wire_gen.go compiles successfully";
        };

        const analyzeProviders = async () => {
            const wireGoContent = await Bun.file(
                "internal/wire/wire.go",
            ).text();
            const wireGenContent = await Bun.file(
                "internal/wire/wire_gen.go",
            ).text();

            const wireGoProviders = (
                wireGoContent.match(/func provide\w+\(/g) || []
            ).map((m) => m.replace("func ", "").replace("(", ""));

            const wireGenProviders = (
                wireGenContent.match(/provide\w+\(/g) || []
            ).map((m) => m.replace("provide", ""));

            const missingProviders = wireGoProviders.filter(
                (p) => !wireGenProviders.some((gp) => gp.includes(p)),
            );

            if (missingProviders.length > 0) {
                return {
                    wireGoCount: wireGoProviders.length,
                    wireGenCount: wireGenProviders.length,
                    missingProviders,
                    message: `Warning: ${missingProviders.length} providers in wire.go may not be in wire_gen.go:\n  - ${missingProviders.join("\n  - ")}`,
                };
            }

            return {
                wireGoCount: wireGoProviders.length,
                wireGenCount: wireGenProviders.length,
                missingProviders: [],
                message: `All ${wireGoProviders.length} providers are present in wire_gen.go`,
            };
        };

        try {
            results.push("Checking wire configuration...");
            results.push(await checkWireGo());

            if (!force) {
                results.push(await checkStaleness());
            } else {
                results.push("Force mode enabled - regenerating wire");
            }

            results.push("Running wire generation...");
            const wireOutput = await runWire();
            results.push(wireOutput);

            results.push("Validating wire_gen.go compilation...");
            results.push(await validateCompilation());

            results.push("Analyzing providers...");
            const providerAnalysis = await analyzeProviders();
            results.push(
                `Found ${providerAnalysis.wireGoCount} providers in wire.go, ${providerAnalysis.wireGenCount} in wire_gen.go`,
            );
            results.push(providerAnalysis.message);

            return results.join("\n");
        } catch (error: any) {
            results.push(`Error: ${error.message}`);
            return results.join("\n");
        }
    },
});
