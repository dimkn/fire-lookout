import { client } from './client'
import type { components } from './schema.gen'

export type SystemOverview = components['schemas']['SystemOverview']
export type StatusItem = components['schemas']['StatusItem']
export type Feed = components['schemas']['Feed']
export type Indicator = components['schemas']['Indicator']
export type Status = components['schemas']['Status']
export type ApiErrorBody = components['schemas']['Error']

/** The create payload, straight from the contract — no hand-written shape to drift. */
export type SubscribeFeedRequest = components['schemas']['SubscribeFeedRequest']

/**
 * A request that did not succeed. serverMessage is the contract's Error.message when the
 * backend sent one — it is written for humans, so callers may show it verbatim.
 */
export class RequestError extends Error {
  constructor(
    readonly status: number,
    readonly code: string | null,
    readonly serverMessage: string | null,
    message: string,
  ) {
    super(message)
    this.name = 'RequestError'
  }
}

// openapi-fetch fills `error` only when the failure body parsed as the contract's Error
// schema. A dead backend behind the dev proxy answers 500 with an empty body, leaving both
// `data` and `error` undefined — so the response itself decides success, and a missing body
// is a failure. Never turn one into an empty result: callers would render "nothing" as if
// the server had said so.
function fail(context: string, response: Response, error: ApiErrorBody | undefined): RequestError {
  const detail = error?.code ? `${error.code}: ${error.message}` : `HTTP ${response.status}`
  return new RequestError(
    response.status,
    error?.code ?? null,
    error?.message ?? null,
    `${context}: ${detail}`,
  )
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

/**
 * Subscribes to a new feed. The backend validates the input and fetches the URL once to
 * prove it is really a feed, so this can fail with a message worth showing the user.
 *
 * Cadence travels as `refresh_interval_sec` — seconds, the same unit the database and the
 * poller use, so nothing has to convert anything.
 */
export async function subscribeFeed(input: SubscribeFeedRequest): Promise<Feed> {
  const { data, error, response } = await client.POST('/feeds', { body: input })
  if (error || !response.ok || !data) {
    throw fail('add integration', response, error)
  }
  return data
}
