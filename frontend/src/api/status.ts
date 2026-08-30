import { client } from './client'
import type { components } from './schema.gen'

export type SystemOverview = components['schemas']['SystemOverview']
export type StatusItem = components['schemas']['StatusItem']
export type Indicator = components['schemas']['Indicator']
export type Status = components['schemas']['Status']
export type ApiError = components['schemas']['Error']

function toError(context: string, error: ApiError | undefined): Error {
  const detail = error ? `${error.code}: ${error.message}` : 'unexpected response'
  return new Error(`${context}: ${detail}`)
}

/** Every subscribed system with its latest known status — one request, no parameters. */
export async function fetchOverview(): Promise<SystemOverview[]> {
  const { data, error } = await client.GET('/overview')
  if (error) {
    throw toError('fetch overview', error)
  }
  return data ?? []
}

/** One system's incidents, newest first. Called lazily, when a card is expanded. */
export async function fetchFeedItems(feedId: number): Promise<StatusItem[]> {
  const { data, error } = await client.GET('/feeds/{feedId}/items', {
    params: { path: { feedId } },
  })
  if (error) {
    throw toError(`fetch items for feed ${feedId}`, error)
  }
  return data ?? []
}
