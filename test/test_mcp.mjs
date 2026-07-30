import path from "path";
import { fileURLToPath } from "url";
import chalk from "chalk";

// 1. Get absolute path of current script (test_mcp.mjs)
const __filename = fileURLToPath(import.meta.url);

// 2. Get directory of current script (test/)
const __dirname = path.dirname(__filename);

// 3. Target files placed in test/ directory
const TARGET_FILE = path.resolve(__dirname + "/tmp", "tmp.txt");
const TARGET_DIR = __dirname;

const BASE_URL = process.env.MIMI_BASE_URL || "http://localhost:2337/";

// 1. Write as plain JS object
const rawObject = {
  status: "success",
  meta: {
    version: "1.0.0",
    symbols: "!@#$%^&*()_+{}[]|\\:;\"'<>,.?/~`",
  },
};

// 2. Dynamically convert to formatted string with indentation
const TEST_CONTENT = JSON.stringify(rawObject, null, 2);

// 🎯 Configure all tools and parameters to test here
const testCases = [
  // ========== write tool ==========
  {
    label: "write (file)",
    name: "write",
    arguments: {
      path: TARGET_FILE,
      content: TEST_CONTENT,
    },
  },
  {
    label: "write (directory) - expected error",
    name: "write",
    arguments: {
      path: TARGET_DIR,
      content: TEST_CONTENT,
    },
  },
  {
    label: "write (missing content) - expected error",
    name: "write",
    arguments: {
      path: TARGET_FILE,
    },
  },
  {
    label: "write (missing path) - expected error",
    name: "write",
    arguments: {
      content: TEST_CONTENT,
    },
  },

  // ========== read tool ==========
  {
    label: "read (file - full)",
    name: "read",
    arguments: {
      path: TARGET_FILE,
    },
  },
  {
    label: "read (file - lines 1~3)",
    name: "read",
    arguments: {
      path: TARGET_FILE,
      start_line: 1,
      end_line: 3,
    },
  },
  {
    label: "read (directory)",
    name: "read",
    arguments: {
      path: TARGET_DIR,
    },
  },
  {
    label: "read (directory - lines 1~3)",
    name: "read",
    arguments: {
      path: TARGET_DIR,
      start_line: 1,
      end_line: 3,
    },
  },
  {
    label: "read (non-existent) - expected error",
    name: "read",
    arguments: {
      path: "/nonexistent/path/to/file.txt",
    },
  },
  {
    label: "read (invalid line range) - expected error",
    name: "read",
    arguments: {
      path: TARGET_FILE,
      start_line: 10,
      end_line: 1,
    },
  },
  {
    label: "read (line out of bounds) - expected error",
    name: "read",
    arguments: {
      path: TARGET_FILE,
      start_line: 999,
    },
  },
  {
    label: "read (missing path) - expected error",
    name: "read",
    arguments: {},
  },

  // ========== edit tool ==========
  {
    label: "edit (replace)",
    name: "edit",
    arguments: {
      path: TARGET_FILE,
      old_string: "success",
      new_string: "completed",
    },
  },
  {
    // Whitespace-insensitive match: file has indentation, but old/new have no whitespace,
    // match after normalizedLine removes all spaces, tabs, newlines.
    label: "edit (whitespace-insensitive match: file has indentation, old has no whitespace)",
    name: "edit",
    arguments: {
      path: TARGET_FILE,
      old_string: '"status":"completed"',
      new_string: '"status":"finished"',
    },
  },
  {
    // Reset file to known state after the previous fuzzy-match edit mangled the
    // formatting (fuzzy match replaces entire lines including indentation and
    // trailing newline). This ensures the next edit test starts from clean JSON.
    label: "write (reset file for whitespace edit tests)",
    name: "write",
    arguments: {
      path: TARGET_FILE,
      content: JSON.stringify(
        {
          status: "finished",
          meta: {
            version: "1.0.0",
            symbols: "!@#$%^&*()_+{}[]|\\:;\"'<>,.?/~`",
          },
        },
        null,
        2,
      ),
    },
  },
  {
    // Whitespace-insensitive match: old has indentation, new changes indentation,
    // match after ignoring all whitespace, write preserves new_string's original indentation.
    // old_string uses 4-space indent; file has 2-space indent → exact match fails,
    // fuzzy match normalizes whitespace and finds the line with similarity 1.0.
    label: "edit (whitespace-insensitive match: old has whitespace, new changes whitespace)",
    name: "edit",
    arguments: {
      path: TARGET_FILE,
      old_string: '    "status": "finished",',
      new_string: '  "status":"done"',
    },
  },
  {
    // old/new both have no whitespace, file has: '"version": "1.0.0"'
    // still matches after removing all whitespace (old -> '"version":"1.0.0"').
    label: "edit (whitespace-insensitive match: no whitespace change version field)",
    name: "edit",
    arguments: {
      path: TARGET_FILE,
      old_string: '"version":"1.0.0",',
      new_string: '"version":"2.0.0",',
    },
  },
  {
    label: "edit (not found) - expected error",
    name: "edit",
    arguments: {
      path: TARGET_FILE,
      old_string: "nonexistent_string_xyz",
      new_string: "replacement",
    },
  },
  {
    label: "edit (same string) - expected error",
    name: "edit",
    arguments: {
      path: TARGET_FILE,
      old_string: "completed",
      new_string: "completed",
    },
  },
  {
    label: "edit (non-existent) - expected error",
    name: "edit",
    arguments: {
      path: "/nonexistent/path/to/file.txt",
      old_string: "old",
      new_string: "new",
    },
  },
  {
    label: "edit (missing path) - expected error",
    name: "edit",
    arguments: {
      old_string: "old",
      new_string: "new",
    },
  },
  {
    label: "edit (missing new_string) - expected error",
    name: "edit",
    arguments: {
      path: TARGET_FILE,
      old_string: "old",
    },
  },
  // ========== tree tool ==========
  {
    label: "tree (default depth)",
    name: "tree",
    arguments: { path: TARGET_DIR },
  },
  {
    label: "tree (depth 1)",
    name: "tree",
    arguments: { path: TARGET_DIR, max_depth: 1 },
  },
  {
    label: "tree (depth 10)",
    name: "tree",
    arguments: { path: TARGET_DIR, max_depth: 10 },
  },
  {
    label: "tree (depth string 10) - verify string-to-number conversion",
    name: "tree",
    arguments: { path: TARGET_DIR, max_depth: "10" },
  },
  {
    label: "tree (non-existent) - expected error",
    name: "tree",
    arguments: { path: "/nonexistent/path" },
  },
  {
    label: "tree (missing path) - expected error",
    name: "tree",
    arguments: {},
  },

  // ========== search_name tool ==========
  {
    label: "search_name (test)",
    name: "search_name",
    arguments: {
      path: TARGET_DIR,
      pattern: "test",
    },
  },
  {
    label: "search_name (mjs)",
    name: "search_name",
    arguments: {
      path: TARGET_DIR,
      pattern: ".mjs",
    },
  },
  {
    label: "search_name (mode: regex)",
    name: "search_name",
    arguments: {
      path: TARGET_DIR,
      pattern: ".*\\.mjs$",
      mode: "regex",
    },
  },
  {
    label: "search_name (mode: regex via string)",
    name: "search_name",
    arguments: {
      path: TARGET_DIR,
      pattern: ".*\\.mjs$",
      mode: "regex",
    },
  },
  {
    label: "search_name (case_sensitive string) - verify string boolean conversion",
    name: "search_name",
    arguments: {
      path: TARGET_DIR,
      pattern: "MJS",
      case_sensitive: "false",
    },
  },

    {
    label: "search_name (glob)",
    name: "search_name",
    arguments: {
      path: TARGET_DIR,
      pattern: "*.mjs",
      mode: "glob",
    },
  },
  {
    label: "search_name (missing path) - expected error",
    name: "search_name",
    arguments: { pattern: "test" },
  },
  {
    label: "search_name (missing pattern) - expected error",
    name: "search_name",
    arguments: { path: TARGET_DIR },
  },

// ========== search_content tool ==========
  {
    label: "search_content (substring)",
    name: "search_content",
    arguments: {
      path: TARGET_DIR,
      pattern: "testCases",
      mode: "substring",
    },
  },
  {
    label: "search_content (mcp)",
    name: "search_content",
    arguments: {
      path: TARGET_DIR,
      pattern: "mcp",
    },
  },
  {
    label: "search_content (mode: regex)",
    name: "search_content",
    arguments: {
      path: TARGET_DIR,
      pattern: "testCases",
      mode: "regex",
    },
  },
  {
    label: "search_content (mode: regex via string)",
    name: "search_content",
    arguments: {
      path: TARGET_DIR,
      pattern: "testCases",
      mode: "regex",
    },
  },
  {
    label: "search_content (case_sensitive string) - verify string boolean conversion",
    name: "search_content",
    arguments: {
      path: TARGET_DIR,
      pattern: "MCP",
      case_sensitive: "false",
    },
  },
  {
    label: "search_content (missing path) - expected error",
    name: "search_content",
    arguments: { pattern: "test" },
  },
  {
    label: "search_content (missing pattern) - expected error",
    name: "search_content",
    arguments: { path: TARGET_DIR },
  },

  // ========== exec tool ==========
  {
    label: "exec (echo)",
    name: "exec",
    arguments: {
      command: "echo 'hello world'",
    },
  },
  {
    label: "exec (ls)",
    name: "exec",
    arguments: {
      command: "ls -la " + TARGET_DIR,
    },
  },
  {
    label: "exec (&&)",
    name: "exec",
    arguments: {
      command: "echo 'step1' && echo 'step2' && echo 'step3'",
    },
  },
  {
    label: "exec (;)",
    name: "exec",
    arguments: {
      command: "echo 'cmd1'; echo 'cmd2'; echo 'cmd3'",
    },
  },
  {
    label: "exec (||)",
    name: "exec",
    arguments: {
      command: "ls /nonexistent || echo 'fallback'",
    },
  },
  {
    label: "exec (|)",
    name: "exec",
    arguments: {
      command: "ls -la " + TARGET_DIR + " | head -5",
    },
  },
  {
    label: "exec (&)",
    name: "exec",
    arguments: {
      command: "sleep 1 & echo 'background'",
    },
  },
  {
    label: "exec (failure) - verify exit code",
    name: "exec",
    arguments: {
      command: "ls /nonexistent 2>&1",
    },
  },
  {
    label: "exec (wait_seconds string) - verify string-to-number conversion",
    name: "exec",
    arguments: {
      command: "echo 'hello'",
      wait_seconds: "30",
    },
  },
  {
    label: "exec (missing command) - expected error",
    name: "exec",
    arguments: {},
  },

  // ========== Cleanup ==========
  {
    label: "cleanup (rm)",
    name: "exec",
    arguments: {
      command: "rm -f " + TARGET_FILE,
    },
  },
];

// Parse command line arguments
const args = process.argv.slice(2);
const VERBOSE_ALL = args.includes("-vv") || args.includes("--verbose-all");
const VERBOSE_FAILURES =
  args.includes("-v") || args.includes("--verbose") || VERBOSE_ALL;

// Helper function to extract result text
function extractResultText(output) {
  if (output.includes('"text":"')) {
    const textMatch = output.match(/"text":"([^"]*")/);
    if (textMatch) {
      return textMatch[1]
        .replace(/\\n/g, "\n")
        .replace(/\\t/g, "\t")
        .replace(/\\\\/g, "\\");
    }
  }
  return output;
}

// Pretty print JSON
function formatJson(obj) {
  return JSON.stringify(obj, null, 2);
}

// Parse response output (handle SSE format and formatting)
function formatResponse(output) {
  // Extract content after SSE data:
  const lines = output
    .split("\n")
    .map((line) => {
      if (line.startsWith("data: ")) {
        return line.slice(6);
      }
      return line;
    })
    .filter((line) => line.trim());

  // Try to parse each data as JSON
  const formatted = lines
    .map((line) => {
      try {
        const parsed = JSON.parse(line);
        return formatJson(parsed);
      } catch {
        return line;
      }
    })
    .join("\n");

  return formatted;
}

async function runMcpTests() {
  const startTime = Date.now();

  console.log(chalk.bold.cyan("\n🚀 Go mimi-tools MCP Automated Batch Test"));
  console.log(chalk.gray("━".repeat(60)));

  if (VERBOSE_ALL) {
    console.log(chalk.yellow("🔍 Verbose mode: show all request/response details"));
  } else if (VERBOSE_FAILURES) {
    console.log(chalk.yellow("🔍 Verbose mode: show only failed request/response details"));
  }

  // ========================================
  // 1. Send initialize to establish session
  // ========================================
  const initPayload = {
    jsonrpc: "2.0",
    id: "init-1",
    method: "initialize",
    params: {
      protocolVersion: "2024-11-05",
      capabilities: {},
      clientInfo: { name: "test-client", version: "1.0.0" },
    },
  };

  const initResponse = await fetch(BASE_URL, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json, text/event-stream",
    },
    body: JSON.stringify(initPayload),
  });

  if (!initResponse.ok) {
    console.log(chalk.red(`❌ Initialization failed: ${initResponse.status}`));
    return;
  }

  const sessionId = initResponse.headers.get("mcp-session-id");
  if (!sessionId) {
    console.log(chalk.red("❌ Failed to get mcp-session-id"));
    return;
  }
  console.log(chalk.green("✅ Session handshake successful"));

  const sessionUrl = `${BASE_URL}?sessionId=${sessionId}`;
  const sessionHeaders = {
    "Content-Type": "application/json",
    Accept: "application/json, text/event-stream",
    "mcp-session-id": sessionId,
  };

  // ========================================
  // 2. Send initialized notification
  // ========================================
  await fetch(sessionUrl, {
    method: "POST",
    headers: sessionHeaders,
    body: JSON.stringify({
      jsonrpc: "2.0",
      method: "notifications/initialized",
    }),
  });

  // ========================================
  // 3. Loop through array, test tools sequentially
  // ========================================
  let passed = 0;
  let failed = 0;

  console.log(chalk.yellow("\n⚡ Running test queue..."));

  for (let i = 0; i < testCases.length; i++) {
    const tc = testCases[i];
    const toolPayload = {
      jsonrpc: "2.0",
      id: `tool-req-${i}`,
      method: "tools/call",
      params: {
        name: tc.name,
        arguments: tc.arguments,
      },
    };

    try {
      const toolResponse = await fetch(sessionUrl, {
        method: "POST",
        headers: sessionHeaders,
        body: JSON.stringify(toolPayload),
      });

      let output = "";
      let hasError = false;

      if (!toolResponse.ok) {
        hasError = true;
      } else {
        const reader = toolResponse.body
          .pipeThrough(new TextDecoderStream())
          .getReader();

        while (true) {
          const { value, done } = await reader.read();
          if (done) break;
          output += value.trim() + "\n";
        }
      }

      const shouldExpectError =
        tc.label.includes("expected error") || tc.label.includes("verify exit code");
      const isExpectedBehavior = shouldExpectError
        ? hasError ||
          output.includes("Error:") ||
          output.includes("Exit Code")
        : !hasError && !output.includes("Error:");

      // Format output
      const toolName = chalk.blue(tc.name);
      const label = chalk.white(tc.label);
      const status = isExpectedBehavior
        ? chalk.green("✔ PASS")
        : chalk.red("✖ FAIL");

      console.log(` ${toolName.padEnd(16)} ${label.padEnd(28)} ${status}`);

      // Verbose output
      const shouldShowVerbose =
        VERBOSE_ALL || (VERBOSE_FAILURES && !isExpectedBehavior);
      if (shouldShowVerbose) {
        console.log(chalk.gray(" ┌─────────────────────────────────"));
        console.log(chalk.cyan(" │ 📤 Request:"));
        console.log(
          chalk.gray(" │ " + formatJson(toolPayload).split("\n").join("\n │ ")),
        );
        console.log(chalk.gray(" ├─────────────────────────────────"));
        console.log(chalk.magenta(" │ 📥 Response:"));
        console.log(
          chalk.gray(" │ " + formatResponse(output).split("\n").join("\n │ ")),
        );
        console.log(chalk.gray(" └─────────────────────────────────"));
      }

      if (isExpectedBehavior) {
        passed++;
      } else {
        failed++;
      }
    } catch (error) {
      console.log(
        ` ${chalk.blue(tc.name).padEnd(16)} ${chalk.white(tc.label).padEnd(28)} ${chalk.red("✖ Error")}`,
      );
      failed++;

      // Verbose output (error case)
      if (VERBOSE_FAILURES) {
        console.log(chalk.gray(" ┌─────────────────────────────────"));
        console.log(chalk.cyan(" │ 📤 Request:"));
        console.log(
          chalk.gray(" │ " + formatJson(toolPayload).split("\n").join("\n │ ")),
        );
        console.log(chalk.gray(" ├─────────────────────────────────"));
        console.log(chalk.red(" │ ❌ Error: " + error.message));
        console.log(chalk.gray(" └─────────────────────────────────"));
      }
    }
  }

  // ========================================
  // 4. Output test summary
  // ========================================
  const duration = ((Date.now() - startTime) / 1000).toFixed(2);

  console.log(chalk.gray("\n" + "━".repeat(60)));
  console.log(chalk.bold.cyan("📊 Test Summary"));
  console.log(chalk.gray("━".repeat(60)));
  console.log(chalk.green(` ✅ Passed: ${passed}`));
  console.log(
    failed > 0
      ? chalk.red(` ❌ Failed: ${failed}`)
      : chalk.gray(` ✅ Failed: ${failed}`),
  );
  console.log(chalk.white(` 📈 Total: ${testCases.length}`));
  console.log(chalk.gray(` ⏱️ Duration: ${duration}s`));
  console.log(chalk.gray("━".repeat(60) + "\n"));
}

runMcpTests().catch(console.error);