import type * as Monaco from "monaco-editor";
import type { RpcMessage } from "@taplo/lsp";

type LspPosition = { line: number; character: number };
type LspRange = { start: LspPosition; end: LspPosition };
type LspTextEdit = { range: LspRange; newText: string };
type LspDiagnostic = {
  range: LspRange;
  severity?: number;
  message: string;
  source?: string;
  code?: string | number;
};

const documentUri = "file:///workspace/enbu.toml";
const schemaUri = "file:///enbu.schema.json";

function lspPosition(position: Monaco.Position): LspPosition {
  return { line: position.lineNumber - 1, character: position.column - 1 };
}

function monacoRange(range: LspRange): Monaco.IRange {
  return {
    startLineNumber: range.start.line + 1,
    startColumn: range.start.character + 1,
    endLineNumber: range.end.line + 1,
    endColumn: range.end.character + 1,
  };
}

function markdown(value: unknown): Monaco.IMarkdownString | Monaco.IMarkdownString[] | undefined {
  if (typeof value === "string") return { value };
  if (value && typeof value === "object" && "value" in value) {
    return { value: String((value as { value: unknown }).value) };
  }
  if (Array.isArray(value)) {
    return value.flatMap((item) => {
      const result = markdown(item);
      return Array.isArray(result) ? result : result ? [result] : [];
    });
  }
  return undefined;
}

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" || typeof value === "number" ? String(value) : undefined;
}

export type TomlLanguageSession = {
  dispose: () => void;
  format: () => Promise<void>;
};

export async function startTomlLanguageSession(
  monaco: typeof Monaco,
  model: Monaco.editor.ITextModel,
  onDiagnostics: (errors: number) => void,
): Promise<TomlLanguageSession> {
  monaco.languages.register({ id: "toml", extensions: [".toml"] });
  monaco.languages.setMonarchTokensProvider("toml", {
    tokenizer: {
      root: [
        [/^\s*\[\[?.*?\]\]?/, "type.identifier"],
        [/#.*$/, "comment"],
        [/\b(true|false)\b/, "keyword"],
        [/-?\d+(?:\.\d+)?/, "number"],
        [/"([^"\\]|\\.)*"/, "string"],
        [/'[^']*'/, "string"],
        [/^[\w.-]+(?=\s*=)/, "key"],
      ],
    },
  });

  const worker = new Worker(new URL("./taplo.worker.ts", import.meta.url), { type: "module" });
  let nextId = 1;
  let version = 1;
  const pending = new Map<number, (result: unknown) => void>();

  const request = <T>(method: string, params: unknown): Promise<T> =>
    new Promise((resolve) => {
      const id = nextId++;
      pending.set(id, resolve as (result: unknown) => void);
      worker.postMessage({ jsonrpc: "2.0", id, method, params } satisfies RpcMessage);
    });

  const notify = (method: string, params: unknown) => {
    worker.postMessage({ jsonrpc: "2.0", method, params } satisfies RpcMessage);
  };

  const ready = new Promise<void>((resolve) => {
    worker.addEventListener("message", (event: MessageEvent<RpcMessage>) => {
      if (event.data.method === "enbu/ready") resolve();
    });
  });

  worker.addEventListener("message", (event: MessageEvent<RpcMessage>) => {
    const message = event.data;
    if (message.method && message.id != null) {
      const result =
        message.method === "workspace/configuration"
          ? []
          : message.method === "workspace/workspaceFolders"
            ? [{ uri: "file:///workspace", name: "enbu" }]
            : null;
      worker.postMessage({ jsonrpc: "2.0", id: message.id, result } satisfies RpcMessage);
      return;
    }
    if (typeof message.id === "number" && ("result" in message || "error" in message)) {
      pending.get(message.id)?.(message.result);
      pending.delete(message.id);
      return;
    }
    if (message.method === "textDocument/publishDiagnostics") {
      const diagnostics = (message.params as { diagnostics: LspDiagnostic[] }).diagnostics;
      const markers = diagnostics.map((diagnostic) => ({
        ...monacoRange(diagnostic.range),
        severity:
          diagnostic.severity === 1
            ? monaco.MarkerSeverity.Error
            : diagnostic.severity === 2
              ? monaco.MarkerSeverity.Warning
              : monaco.MarkerSeverity.Info,
        message: diagnostic.message,
        source: diagnostic.source ?? "taplo",
        code: diagnostic.code == null ? undefined : String(diagnostic.code),
      }));
      monaco.editor.setModelMarkers(model, "taplo", markers);
      onDiagnostics(
        markers.filter((marker) => marker.severity === monaco.MarkerSeverity.Error).length,
      );
    }
  });

  await ready;
  await request("initialize", {
    processId: null,
    rootUri: "file:///workspace",
    capabilities: {
      textDocument: {
        completion: { completionItem: { snippetSupport: true, documentationFormat: ["markdown"] } },
        hover: { contentFormat: ["markdown"] },
        publishDiagnostics: {},
        formatting: {},
      },
      workspace: { configuration: false },
    },
    workspaceFolders: [{ uri: "file:///workspace", name: "enbu" }],
  });
  notify("initialized", {});
  notify("taplo/associateSchema", {
    document_uri: documentUri,
    schema_uri: schemaUri,
    rule: { url: documentUri },
    priority: 100,
  });
  notify("textDocument/didOpen", {
    textDocument: { uri: documentUri, languageId: "toml", version, text: model.getValue() },
  });

  const modelSubscription = model.onDidChangeContent(() => {
    version += 1;
    notify("textDocument/didChange", {
      textDocument: { uri: documentUri, version },
      contentChanges: [{ text: model.getValue() }],
    });
  });

  const completion = monaco.languages.registerCompletionItemProvider("toml", {
    triggerCharacters: ["=", '"', "."],
    async provideCompletionItems(_model, position) {
      const result = await request<
        { items: Array<Record<string, unknown>> } | Array<Record<string, unknown>> | null
      >("textDocument/completion", {
        textDocument: { uri: documentUri },
        position: lspPosition(position),
      });
      const items = Array.isArray(result) ? result : (result?.items ?? []);
      const word = model.getWordUntilPosition(position);
      const defaultRange = new monaco.Range(
        position.lineNumber,
        word.startColumn,
        position.lineNumber,
        word.endColumn,
      );
      return {
        suggestions: items.map((item) => ({
          label: stringValue(item.label) ?? "",
          kind: monaco.languages.CompletionItemKind.Property,
          detail: stringValue(item.detail),
          documentation: (() => {
            const value = markdown(item.documentation);
            return Array.isArray(value) ? value[0] : value;
          })(),
          insertText: stringValue(item.insertText) ?? stringValue(item.label) ?? "",
          insertTextRules:
            item.insertTextFormat === 2
              ? monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet
              : undefined,
          range: (item.textEdit as LspTextEdit | undefined)?.range
            ? monacoRange((item.textEdit as LspTextEdit).range)
            : defaultRange,
        })),
      };
    },
  });

  const hover = monaco.languages.registerHoverProvider("toml", {
    async provideHover(_model, position) {
      const result = await request<{ contents: unknown; range?: LspRange } | null>(
        "textDocument/hover",
        { textDocument: { uri: documentUri }, position: lspPosition(position) },
      );
      if (!result) return null;
      const contents = markdown(result.contents);
      return {
        contents: Array.isArray(contents) ? contents : contents ? [contents] : [],
        range: result.range ? monacoRange(result.range) : undefined,
      };
    },
  });

  const formattingEdits = async () => {
    const edits = await request<LspTextEdit[] | null>("textDocument/formatting", {
      textDocument: { uri: documentUri },
      options: { tabSize: 2, insertSpaces: true, trimTrailingWhitespace: true },
    });
    return (edits ?? []).map((edit) => ({ range: monacoRange(edit.range), text: edit.newText }));
  };

  const formatting = monaco.languages.registerDocumentFormattingEditProvider("toml", {
    provideDocumentFormattingEdits() {
      return formattingEdits();
    },
  });

  return {
    async format() {
      const edits = await formattingEdits();
      if (edits?.length) model.pushEditOperations([], edits, () => null);
    },
    dispose() {
      notify("textDocument/didClose", { textDocument: { uri: documentUri } });
      modelSubscription.dispose();
      completion.dispose();
      hover.dispose();
      formatting.dispose();
      monaco.editor.setModelMarkers(model, "taplo", []);
      worker.terminate();
    },
  };
}
