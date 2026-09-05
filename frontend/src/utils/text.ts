const ELLIPSIS = '...'

/**
 * Shortens text to maxLength characters, spending the last three on an ellipsis so the
 * result never exceeds the limit. Text at or under the limit is returned untouched.
 */
export function truncate(text: string, maxLength: number): string {
  if (text.length <= maxLength) {
    return text
  }
  return text.slice(0, maxLength - ELLIPSIS.length) + ELLIPSIS
}
