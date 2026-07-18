export type RegisterCleanup = (cleanup: () => void) => void;

function disposeAll(cleanups: Array<() => void>): void {
  let firstError: unknown;
  for (const cleanup of cleanups.splice(0).reverse()) {
    try {
      cleanup();
    } catch (error) {
      firstError ??= error;
    }
  }
  if (firstError !== undefined) throw firstError;
}

export class SessionController {
  private activeCleanups: Array<() => void> = [];
  private starting = false;

  async restart(start: (register: RegisterCleanup) => Promise<void>): Promise<boolean> {
    if (this.starting) return false;
    this.starting = true;
    const pendingCleanups: Array<() => void> = [];

    try {
      disposeAll(this.activeCleanups);
      await start((cleanup) => pendingCleanups.push(cleanup));
      this.activeCleanups = pendingCleanups;
      return true;
    } catch (error) {
      try {
        disposeAll(pendingCleanups);
      } catch (cleanupError) {
        throw new AggregateError([error, cleanupError], "Session startup and cleanup failed");
      }
      throw error;
    } finally {
      this.starting = false;
    }
  }

  dispose(): void {
    disposeAll(this.activeCleanups);
  }
}
