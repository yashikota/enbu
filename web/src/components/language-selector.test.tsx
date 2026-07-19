import { afterEach, beforeEach, describe, expect, it } from "vite-plus/test";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";
import { I18nProvider } from "../lib/i18n";
import { LanguageSelector } from "./language-selector";

let container: HTMLDivElement;
let root: ReturnType<typeof createRoot>;

beforeEach(() => {
  localStorage.setItem("enbu_locale", "en");
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  localStorage.removeItem("enbu_locale");
});

describe("LanguageSelector", () => {
  it("renders both locales and switches the shared locale", () => {
    act(() => {
      root.render(
        <I18nProvider>
          <LanguageSelector />
        </I18nProvider>,
      );
    });

    const select = container.querySelector("select");
    expect(container.textContent).toContain("Language");
    expect(Array.from(select?.options ?? []).map((option) => option.text)).toEqual([
      "English",
      "日本語",
    ]);

    act(() => {
      if (!select) return;
      select.value = "ja";
      select.dispatchEvent(new Event("change", { bubbles: true }));
    });

    expect(container.textContent).toContain("言語");
    expect(localStorage.getItem("enbu_locale")).toBe("ja");
  });
});
