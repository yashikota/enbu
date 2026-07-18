import { TaploLsp, type RpcMessage } from "@taplo/lsp";
import schemaText from "../../../enbu.schema.json?raw";

const encoder = new TextEncoder();
const schemaPath = "/enbu.schema.json";

const environment = {
  now: () => new Date(),
  envVar: () => undefined,
  envVars: () => [] as Array<[string, string]>,
  stdErrAtty: () => false,
  stdin: async () => new Uint8Array(),
  stdout: async (bytes: Uint8Array) => bytes.length,
  stderr: async (bytes: Uint8Array) => bytes.length,
  glob: () => [] as string[],
  readFile: async (path: string) => {
    if (path.endsWith(schemaPath)) return encoder.encode(schemaText);
    throw new Error(`Taplo cannot read ${path}`);
  },
  writeFile: async () => {},
  urlToFilePath: (url: string) => new URL(url).pathname,
  isAbsolute: (path: string) => path.startsWith("/") || /^[A-Za-z]:[\\/]/.test(path),
  cwd: () => "/",
  findConfigFile: () => undefined,
};

let server: TaploLsp | undefined;

void TaploLsp.initialize(environment, {
  onMessage(message) {
    globalThis.postMessage(message);
  },
}).then((instance) => {
  server = instance;
  globalThis.postMessage({ jsonrpc: "2.0", method: "enbu/ready" } satisfies RpcMessage);
});

globalThis.addEventListener("message", (event: MessageEvent<RpcMessage>) => {
  server?.send(event.data);
});
