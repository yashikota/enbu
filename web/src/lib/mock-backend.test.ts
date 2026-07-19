import { describe, expect, it } from "vite-plus/test";
import { mockBackend } from "./mock-backend";

describe("mockBackend preview user", () => {
  it("uses yashikota consistently", async () => {
    const [status, initialized, owners, recipients] = await Promise.all([
      mockBackend.authStatus(),
      mockBackend.initialize(),
      mockBackend.listRepositoryOwners(),
      mockBackend.listRecipients(),
    ]);

    expect(status.username).toBe("yashikota");
    expect(initialized.username).toBe("yashikota");
    expect(owners).toContainEqual({ login: "yashikota", organization: false });
    expect(recipients.some((recipient) => recipient.username === "yashikota")).toBe(true);
  });
});
