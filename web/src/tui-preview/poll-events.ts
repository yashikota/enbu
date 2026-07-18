export type PollEvent = { userdata: bigint; type: number };

export function collectPollEvents(
  stdinUserdata: bigint | undefined,
  clockUserdata: bigint | undefined,
  readable: boolean,
): PollEvent[] {
  const events: PollEvent[] = [];
  if (readable && stdinUserdata !== undefined) {
    events.push({ userdata: stdinUserdata, type: 1 });
  }
  if (clockUserdata !== undefined && (!readable || stdinUserdata === undefined)) {
    events.push({ userdata: clockUserdata, type: 0 });
  }
  return events;
}
