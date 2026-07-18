declare module "xterm-pty" {
  type Disposable = { dispose(): void };

  export class Termios {
    constructor(iflag: number, oflag: number, cflag: number, lflag: number, cc: number[]);
    readonly iflag: number;
    readonly oflag: number;
    readonly cflag: number;
    readonly lflag: number;
    readonly cc: number[];
  }

  export type Slave = {
    ioctl(request: "TCGETS"): Termios;
    ioctl(request: "TCSETS", value: Termios): void;
  };

  export type Master = {
    activate(terminal: {
      write(data: string | Uint8Array, callback?: () => void): void;
      onData(listener: (data: string) => void): Disposable;
      onBinary(listener: (data: string) => void): Disposable;
      onResize(listener: (size: { cols: number; rows: number }) => void): Disposable;
    }): void;
    dispose(): void;
  };

  export function openpty(): { master: Master; slave: Slave };

  export class TtyServer {
    constructor(slave: Slave);
    start(worker: Worker, callback?: (event: MessageEvent<unknown>) => void): void;
    stop(): void;
  }

  export const ISTRIP: number;
  export const INLCR: number;
  export const IGNCR: number;
  export const ICRNL: number;
  export const IXON: number;
  export const OPOST: number;
  export const ECHO: number;
  export const ECHONL: number;
  export const ICANON: number;
  export const ISIG: number;
  export const IEXTEN: number;
}
