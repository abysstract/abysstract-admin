import { aRaw } from "./a.raw";
import { bRaw } from "./b.raw";

// Sample topology mirrors generated output: raw modules only flow into this
// static registry; no module registers itself at runtime.
export const bidirRawOperations = {
  A: { list: aRaw.list },
  B: { list: bRaw.list },
} as const;
