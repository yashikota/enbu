import { beforeEach, describe, expect, it } from "vite-plus/test";
import { detectLocale, translate } from "./i18n";

const storage = new Map<string, string>();

beforeEach(() => {
  storage.clear();
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    value: {
      getItem: (key: string) => storage.get(key) ?? null,
      setItem: (key: string, value: string) => storage.set(key, value),
      clear: () => storage.clear(),
    },
  });
  Object.defineProperty(globalThis, "navigator", {
    configurable: true,
    value: { language: "en-US" },
  });
});

describe("i18n", () => {
  it("uses saved locale first", () => {
    localStorage.setItem("enbu_locale", "ja");
    expect(detectLocale()).toBe("ja");
  });

  it("falls back to English for unsupported locale", () => {
    localStorage.setItem("enbu_locale", "fr");
    expect(detectLocale()).toBe("en");
  });

  it("translates with interpolation", () => {
    expect(translate("en", "repo.current", { owner: "octo", repo: "hello" })).toBe(
      "Repository: octo/hello",
    );
    expect(translate("ja", "auth.expiresIn", { seconds: 30 })).toBe("有効期限まで 30 秒");
    expect(translate("ja", "init.gitAction")).toBe("Gitを初期化");
    expect(translate("en", "init.createRemote")).toBe("Create repository");
    expect(translate("ja", "init.repositoryOwner")).toBe("作成先アカウント");
    expect(translate("ja", "dashboard.key")).toBe("名前");
    expect(translate("ja", "dashboard.copyKey")).toBe("名前をコピー");
    expect(translate("ja", "dashboard.keyCopied")).toBe("名前をコピー済み");
  });

  it("translates duplicateKey with key interpolation", () => {
    expect(translate("en", "dashboard.duplicateKey", { key: "MY_SECRET" })).toBe(
      'Key "MY_SECRET" already exists. Edit the existing secret instead.',
    );
    expect(translate("ja", "dashboard.duplicateKey", { key: "MY_SECRET" })).toBe(
      "キー「MY_SECRET」はすでに存在します。既存のシークレットを編集してください。",
    );
  });
});
