// A stand-in for `reactogenic lsp --stdio` in the editor suite `transpiled`:
// a language server from before `reactogenic/transpiled` (RGP1-108) — it
// starts, keeps the documents, and answers any other request with
// MethodNotFound. *Show Transpiled TSX* then says that the server is too old.
import fs from "node:fs";

const documents = new Map();

function send(message) {
  const body = Buffer.from(JSON.stringify({ jsonrpc: "2.0", ...message }), "utf8");
  process.stdout.write(`Content-Length: ${body.length}\r\n\r\n`);
  process.stdout.write(body);
}

function handle(message) {
  const { id, method, params } = message;
  switch (method) {
    case "initialize":
      return send({ id, result: { capabilities: { textDocumentSync: 1 }, serverInfo: { name: "reactogenic", version: "0.0.0-old" } } });
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
    default:
      if (id !== undefined && method) {
        send({ id, error: { code: -32601, message: `no ${method}` } });
      }
  }
}

if (process.argv[2] !== "lsp") {
  fs.writeSync(2, "the old server has `lsp --stdio` only\n");
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
