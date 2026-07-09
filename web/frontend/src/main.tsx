import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider, createRouter } from "@tanstack/react-router";
import { ChakraProvider } from "@chakra-ui/react";
import { routeTree } from "./routeTree.gen";
import { system } from "./theme";
import "./app.css";

const router = createRouter({ routeTree, basepath: import.meta.env.BASE_URL });

// In Wails, target="_blank" opens a new native window instead of the OS browser.
// Intercept all such clicks and route them through the Wails runtime instead.
document.addEventListener("click", (e) => {
  const a = (e.target as Element).closest("a[target='_blank']") as HTMLAnchorElement | null;
  if (!a) return;
  const wailsRuntime = (window as unknown as { runtime?: { BrowserOpenURL?: (u: string) => void } }).runtime;
  if (!wailsRuntime?.BrowserOpenURL) return;
  e.preventDefault();
  wailsRuntime.BrowserOpenURL(a.href);
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ChakraProvider value={system}>
      <RouterProvider router={router} />
    </ChakraProvider>
  </StrictMode>,
);
