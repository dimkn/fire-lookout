import { client } from './client'
import type { components } from './schema.gen'

export type SystemOverview = components['schemas']['SystemOverview']
export type StatusItem = components['schemas']['StatusItem']
export type Indicator = components['schemas']['Indicator']
export type Status = components['schemas']['Status']
export type ApiError = components['schemas']['Error']

// openapi-fetch fills `error` only when the failure body parsed as the contract's Error
// schema. A dead backend behind the dev proxy answers 500 with an empty body, leaving both
// `data` and `error` undefined — so the response itself decides success, and a missing body
// is a failure. Never turn one into an empty result: callers would render "nothing" as if
// the server had said so.
function fail(context: string, response: Response, error: ApiError | undefined): Error {
  const detail = error?.code ? `${error.code}: ${error.message}` : `HTTP ${response.status}`
  return new Error(`${context}: ${detail}`)
}

/** Every subscribed system with its latest known status — one request, no parameters. */
export async function fetchOverview(): Promise<SystemOverview[]> {
  const { data, error, response } = await client.GET('/overview')
  if (error || !response.ok || !data) {
    throw fail('fetch overview', response, error)
  }
  return data
}

/** One system's incidents, newest first. Called lazily, when a card is expanded. */
export async function fetchFeedItems(feedId: number): Promise<StatusItem[]> {
  const { data, error, response } = await client.GET('/feeds/{feedId}/items', {
    params: { path: { feedId } },
  })
  if (error || !response.ok || !data) {
    throw fail(`fetch items for feed ${feedId}`, response, error)
  }
  return data
}
