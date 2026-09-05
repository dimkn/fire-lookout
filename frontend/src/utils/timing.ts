/**
 * How long a loading affordance stays up at minimum. The backend runs on the same machine
 * and answers in single-digit milliseconds, so without a floor the loader would flash for
 * a frame or two — read as a glitch rather than as progress.
 */
export const MIN_LOADING_MS = 500

/** How long a status banner stays up before dismissing itself. */
export const BANNER_TIMEOUT_MS = 5000

/** Resolves after ms milliseconds. */
export function delay(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, ms)
  })
}
