// A stand-in for `reactogenic lsp --stdio` in the editor suite `fake`: a
// language server with nothing but `reactogenic/transpiled`, which the real
// server gains with RGP1-108. It answers as the real one does for a *stopped*
// file (ide.md, *Tolerance*): the source itself, 1:1 — text that is not TSX.
import fs from "node:fs";

const documents = new Map();
let asked = 0;

function send(message) {
  const body = Buffer.from(JSON.stringify({ jsonrpc: "2.0", ...message }), "utf8");
  process.stdout.write(`Content-Length: ${body.length}\r\n\r\n`);
  process.stdout.write(body);
}

function handle(message) {
  const { id, method, params } = message;
  switch (method) {
    case "initialize":
      return send({ id, result: { capabilities: { textDocumentSync: 1 }, serverInfo: { name: "reactogenic", version: "0.0.0-fake" } } });
    case "shutdown":
      return send({ id, result: null });
    case "exit":
      return process.exit(0);
    case "textDocument/didOpen":
      return void documents.set(params.textDocument.uri, params.textDocument.text);
    case "textDocument/didChange":
      return void documents.set(params.textDocument.uri, params.contentChanges.at(-1).text);
    case "textDocument/didClose":
      return void documents.delete(params.textDocument.uri);
    case "reactogenic/transpiled": {
      const text = documents.get(params.textDocument.uri);
      if (text === undefined) {
        return send({ id, error: { code: -32602, message: `${params.textDocument.uri} is not open` } });
      }
      asked++;
      return send({ id, result: { text: `// transpiled ${asked}\n${text}`, step: "stopped: slots" } });
    }
    default:
      if (id !== undefined && method) {
        send({ id, error: { code: -32601, message: `no ${method}` } });
      }
  }
}

if (process.argv[2] !== "lsp") {
  fs.writeSync(2, "the fake server has `lsp --stdio` only\n");
  process.exit(2);
}
let buffer = Buffer.alloc(0);
process.stdin.on("data", (chunk) => {
  buffer = Buffer.concat([buffer, chunk]);
  for (;;) {
    const head = buffer.indexOf("\r\n\r\n");
    if (head < 0) {
      return;
    }
    const length = Number(/Content-Length: (\d+)/i.exec(buffer.subarray(0, head).toString())[1]);
    if (buffer.length < head + 4 + length) {
      return;
    }
    const body = buffer.subarray(head + 4, head + 4 + length).toString("utf8");
    buffer = buffer.subarray(head + 4 + length);
    handle(JSON.parse(body));
  }
});
process.stdin.on("end", () => process.exit(0));
