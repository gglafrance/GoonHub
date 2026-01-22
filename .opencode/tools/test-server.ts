import { tool } from "@opencode-ai/plugin"
import { writeFileSync, readFileSync, existsSync, copyFileSync } from "fs"

const DEFAULT_CONFIG_PATH = "/home/mathis/Documents/Dev/gh/.opencode/tools/test-config.yaml"
const TEST_CONFIG_PATH = "/tmp/goonhub-test-config.yaml"
const TEST_LOG_PATH = "/tmp/goonhub-test.log"
const SERVER_BINARY = "goonhub"

async function runCommand(cmd: string, cwd?: string): Promise<{ stdout: string; stderr: string; exitCode: number }> {
  const proc = Bun.spawn({
    cmd: ["sh", "-c", cmd],
    cwd: cwd || process.cwd(),
    stdout: "pipe",
    stderr: "pipe",
  })

  const [stdout, stderr] = await Promise.all([
    new Response(proc.stdout).text(),
    new Response(proc.stderr).text(),
  ])

  const exitCode = await proc.exited
  return { stdout, stderr, exitCode }
}

function generateConfigYAML(config: Record<string, any>): string {
  const yamlContent: string[] = []

  if (config.environment) yamlContent.push(`environment: ${config.environment}`)
  if (config.server) {
    yamlContent.push("server:")
    if (config.server.port) yamlContent.push(`  port: "${config.server.port}"`)
    if (config.server.allowed_origins && config.server.allowed_origins.length > 0) {
      yamlContent.push("  allowed_origins:")
      config.server.allowed_origins.forEach((origin: string) => {
        yamlContent.push(`    - "${origin}"`)
      })
    }
  }
  if (config.database) {
    yamlContent.push("database:")
    if (config.database.driver) yamlContent.push(`  driver: ${config.database.driver}`)
    if (config.database.source) yamlContent.push(`  source: ${config.database.source}`)
  }
  if (config.log) {
    yamlContent.push("log:")
    if (config.log.level) yamlContent.push(`  level: ${config.log.level}`)
    if (config.log.format) yamlContent.push(`  format: ${config.log.format}`)
  }
  if (config.processing) {
    yamlContent.push("processing:")
    if (config.processing.frame_interval) yamlContent.push(`  frame_interval: ${config.processing.frame_interval}`)
    if (config.processing.frame_width) yamlContent.push(`  frame_width: ${config.processing.frame_width}`)
    if (config.processing.frame_height) yamlContent.push(`  frame_height: ${config.processing.frame_height}`)
    if (config.processing.frame_quality) yamlContent.push(`  frame_quality: ${config.processing.frame_quality}`)
    if (config.processing.worker_count) yamlContent.push(`  worker_count: ${config.processing.worker_count}`)
    if (config.processing.thumbnail_seek) yamlContent.push(`  thumbnail_seek: "${config.processing.thumbnail_seek}"`)
    if (config.processing.frame_output_dir) yamlContent.push(`  frame_output_dir: "${config.processing.frame_output_dir}"`)
    if (config.processing.thumbnail_dir) yamlContent.push(`  thumbnail_dir: "${config.processing.thumbnail_dir}"`)
  }
  if (config.auth) {
    yamlContent.push("auth:")
    if (config.auth.paseto_secret) yamlContent.push(`  paseto_secret: "${config.auth.paseto_secret}"`)
    if (config.auth.admin_username) yamlContent.push(`  admin_username: "${config.auth.admin_username}"`)
    if (config.auth.admin_password) yamlContent.push(`  admin_password: "${config.auth.admin_password}"`)
    if (config.auth.token_duration) yamlContent.push(`  token_duration: ${config.auth.token_duration}`)
    if (config.auth.login_rate_limit) yamlContent.push(`  login_rate_limit: ${config.auth.login_rate_limit}`)
    if (config.auth.login_rate_burst) yamlContent.push(`  login_rate_burst: ${config.auth.login_rate_burst}`)
  }

  return yamlContent.join("\n")
}

export default tool({
  description: "Manage test server with default config on port 8081",
  args: {
    action: tool.schema.string().describe("Server action: start, stop, or status"),
    config: tool.schema.string().optional().describe("Custom config as JSON (uses default if not provided)"),
    binaryPath: tool.schema.string().optional().describe("Path to server binary (default: ./goonhub)"),
    useGoRun: tool.schema.boolean().optional().describe("Use go run instead of binary"),
    waitForReady: tool.schema.boolean().optional().describe("Wait for server to be ready"),
    port: tool.schema.string().optional().describe("Port for readiness check (default 8081)"),
    cleanup: tool.schema.boolean().optional().describe("Cleanup database and logs before starting"),
  },
  async execute(args, context) {
    const { action, config, binaryPath = SERVER_BINARY, useGoRun = false, waitForReady = true, port, cleanup = false } = args

    try {
      switch (action) {
        case "start": {
          let serverPort = port || "8081"
          let useDefaultConfig = !config || config.trim().length === 0

          if (cleanup) {
            await runCommand("rm -f library.db library-test.db")
            await runCommand(`rm -f ${TEST_LOG_PATH}`)
          }

          await runCommand("pkill -9 -f goonhub || true")
          await runCommand("sleep 2")

          if (useDefaultConfig) {
            copyFileSync(DEFAULT_CONFIG_PATH, TEST_CONFIG_PATH)
            const defaultConfigYAML = readFileSync(DEFAULT_CONFIG_PATH, "utf-8")
            const match = defaultConfigYAML.match(/port:\s*"(\d+)"/)
            if (match && match[1]) {
              serverPort = match[1]
            }
          } else {
            const configObj = JSON.parse(config)
            const configYAML = generateConfigYAML(configObj)
            writeFileSync(TEST_CONFIG_PATH, configYAML)
            serverPort = configObj.server?.port || serverPort
          }

          const startCmd = useGoRun
            ? `GOONHUB_CONFIG="${TEST_CONFIG_PATH}" go run cmd/server/main.go > ${TEST_LOG_PATH} 2>&1 &`
            : `GOONHUB_CONFIG="${TEST_CONFIG_PATH}" ./${binaryPath} > ${TEST_LOG_PATH} 2>&1 &`

          await runCommand(startCmd)

          let serverReady = false
          let attempts = 0
          const maxAttempts = 30

          if (waitForReady) {
            while (attempts < maxAttempts && !serverReady) {
              const checkResult = await runCommand(`lsof -i :${serverPort} 2>/dev/null | grep LISTEN`)
              if (checkResult.exitCode === 0 && checkResult.stdout.trim().length > 0) {
                serverReady = true
              } else {
                await new Promise(resolve => setTimeout(resolve, 500))
                attempts++
              }
            }
          }

          const pidResult = await runCommand("pgrep -f 'goonhub'")
          const pid = pidResult.stdout.trim().split("\n")[0]

          const results: string[] = []
          results.push(`Status: ${serverReady ? "ready" : "started"}`)
          results.push(`PID: ${pid}`)
          results.push(`Port: ${serverPort}`)
          results.push(`Config: ${TEST_CONFIG_PATH}`)
          results.push(`Log: ${TEST_LOG_PATH}`)
          results.push(`Ready: ${serverReady}`)
          return results.join("\n")
        }

        case "stop": {
          await runCommand("pkill -9 -f goonhub || true")
          await runCommand("sleep 1")

          const pidResult = await runCommand("pgrep -f 'goonhub'")
          const running = pidResult.exitCode === 0

          const results: string[] = []
          results.push(`Status: stopped`)
          results.push(`Running: ${running}`)
          return results.join("\n")
        }

        case "status": {
          const pidResult = await runCommand("pgrep -f 'goonhub'")
          const running = pidResult.exitCode === 0

          let logExists = false
          let logSize = 0
          if (existsSync(TEST_LOG_PATH)) {
            logExists = true
            logSize = readFileSync(TEST_LOG_PATH).length
          }

          const results: string[] = []
          results.push(`Running: ${running}`)
          results.push(`PID: ${pidResult.stdout.trim().split("\n")[0] || null}`)
          results.push(`LogExists: ${logExists}`)
          results.push(`LogSize: ${logSize}`)
          results.push(`LogPath: ${TEST_LOG_PATH}`)
          results.push(`ConfigExists: ${existsSync(TEST_CONFIG_PATH)}`)
          return results.join("\n")
        }

        default:
          throw new Error(`Unknown action: ${action}`)
      }
    } catch (error: any) {
      return `Error: ${error.message}`
    }
  },
})
