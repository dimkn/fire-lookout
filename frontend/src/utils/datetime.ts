// The backend stores and serves every timestamp as RFC3339 UTC, so timestamps are
// rendered in UTC too: a status page is read across timezones, and silently shifting to
// the viewer's local clock makes two people describing "the 09:00 incident" disagree.
const formatter = new Intl.DateTimeFormat('en-GB', {
  dateStyle: 'medium',
  timeStyle: 'short',
  timeZone: 'UTC',
})

/** Formats an RFC3339 timestamp for display, echoing the input back if it is unusable. */
export function formatTimestamp(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) {
    return iso
  }
  return `${formatter.format(date)} UTC`
}
