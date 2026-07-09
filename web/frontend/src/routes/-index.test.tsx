import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";
import { ChakraProvider } from "@chakra-ui/react";
import { system } from "../theme";
import { I18nProvider } from "../lib/i18n";
import { SecretRow } from "./index";

vi.mock("../lib/backend", () => ({
  backend: {
    authStatus: vi.fn(async () => ({ authenticated: false })),
    startDeviceLogin: vi.fn(),
    deviceStatus: vi.fn(),
    cancelDeviceLogin: vi.fn(),
    logout: vi.fn(),
    repoStatus: vi.fn(async () => ({ selected: false })),
    browseRepository: vi.fn(),
    selectRepository: vi.fn(),
    initialize: vi.fn(),
    listEnvironments: vi.fn(async () => []),
    listSecrets: vi.fn(async () => ({ environment: "default", secrets: [] })),
    addSecret: vi.fn(),
    editSecret: vi.fn(),
    deleteSecret: vi.fn(),
    pullSecrets: vi.fn(),
    syncSecrets: vi.fn(),
    listRepositories: vi.fn(async () => []),
    removeRepository: vi.fn(),
    listRecipients: vi.fn(async () => []),
    readConfig: vi.fn(async () => ""),
    writeConfig: vi.fn(),
  },
  openURL: vi.fn(),
}));

let container: HTMLDivElement;
let root: ReturnType<typeof createRoot>;

beforeEach(() => {
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root.unmount();
  });
  container.remove();
});

function renderSecretRow(props: {
  secretKey: string;
  secretValue: string;
  onEdit?: (val: string) => Promise<void>;
  onDelete?: () => Promise<void>;
  deleteLabel?: string;
}) {
  act(() => {
    root.render(
      <ChakraProvider value={system}>
        <I18nProvider>
          <SecretRow
            secretKey={props.secretKey}
            secretValue={props.secretValue}
            onEdit={props.onEdit ?? (async () => {})}
            onDelete={props.onDelete ?? (async () => {})}
            deleteLabel={props.deleteLabel ?? "Delete"}
          />
        </I18nProvider>
      </ChakraProvider>,
    );
  });
}

function queryInput(value: string): HTMLInputElement | null {
  return (
    Array.from(container.querySelectorAll<HTMLInputElement>("input")).find(
      (el) => el.value === value || el.defaultValue === value,
    ) ?? null
  );
}

function queryButton(label: string): HTMLButtonElement | null {
  return (
    Array.from(container.querySelectorAll<HTMLButtonElement>("button")).find(
      (b) => b.textContent?.includes(label) || b.getAttribute("aria-label") === label,
    ) ?? null
  );
}

describe("SecretRow", () => {
  it("renders value input as password by default", () => {
    renderSecretRow({ secretKey: "API_KEY", secretValue: "super-secret" });
    expect(queryInput("super-secret")?.type).toBe("password");
  });

  it("toggles to text type on eye button click", () => {
    renderSecretRow({ secretKey: "API_KEY", secretValue: "super-secret" });
    act(() => {
      queryButton("Show value")?.click();
    });
    expect(queryInput("super-secret")?.type).toBe("text");
    expect(queryButton("Hide value")).toBeTruthy();
  });

  it("toggles back to password on second click", () => {
    renderSecretRow({ secretKey: "API_KEY", secretValue: "super-secret" });
    act(() => {
      queryButton("Show value")?.click();
    });
    act(() => {
      queryButton("Hide value")?.click();
    });
    expect(queryInput("super-secret")?.type).toBe("password");
  });

  it("calls onDelete when delete button is clicked", () => {
    const onDelete = vi.fn(async () => {});
    renderSecretRow({ secretKey: "API_KEY", secretValue: "super-secret", onDelete });
    act(() => {
      queryButton("Delete")?.click();
    });
    expect(onDelete).toHaveBeenCalledTimes(1);
  });

  it("renders key input as readonly", () => {
    renderSecretRow({ secretKey: "API_KEY", secretValue: "super-secret" });
    expect(queryInput("API_KEY")?.readOnly).toBe(true);
  });
});
