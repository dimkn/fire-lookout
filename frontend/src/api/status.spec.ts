import type { Mock } from 'vitest'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { client } from './client'
import { fetchFeedItems, fetchOverview, subscribeFeed } from './status'
import type { Feed, StatusItem, SystemOverview } from './status'

vi.mock('./client', () => ({ client: { GET: vi.fn(), POST: vi.fn() } }))

const get = client.GET as unknown as Mock
const post = client.POST as unknown as Mock

/** A bare response, the way a proxy or a dead backend answers: status only, no body. */
const res = (status: number) => new Response(null, { status })

function makeSystem(): SystemOverview {
  return {
    feed: {
      id: 1,
      url: 'https://a.test/feed.atom',
      title: 'GitHub',
      group_id: 0,
      enabled: true,
      refresh_interval_sec: 300,
      created_at: '2026-08-01T00:00:00Z',
      updated_at: '2026-08-18T09:05:00Z',
    },
    indicator: 'operational',
  }
}

function makeItem(): StatusItem {
  return {
    id: 101,
    feed_id: 1,
    guid: 'guid-101',
    title: 'Incident',
    status: 'resolved',
    fetched_at: '2026-08-18T09:05:00Z',
  }
}

describe('fetchOverview', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('returns the systems the API sent', async () => {
    get.mockResolvedValue({ data: [makeSystem()], response: res(200) })

    await expect(fetchOverview()).resolves.toHaveLength(1)
  })

  it('passes an empty list through — no systems saved yet is a valid answer', async () => {
    get.mockResolvedValue({ data: [], response: res(200) })

    await expect(fetchOverview()).resolves.toEqual([])
  })

  it('throws with the code and message when the API reports an error', async () => {
    get.mockResolvedValue({
      error: { code: 'internal_error', message: 'the request could not be completed' },
      response: res(500),
    })

    await expect(fetchOverview()).rejects.toThrow(/internal_error/)
  })

  // Regression: a dead backend behind the dev proxy answers 500 with an empty body, so
  // openapi-fetch reports neither data nor a parsable error. Reading that as "no systems"
  // would wipe the list on screen, which is exactly what a failed refresh must not do.
  it('throws — never resolves empty — when the response carries no usable body', async () => {
    get.mockResolvedValue({ data: undefined, error: undefined, response: res(500) })

    await expect(fetchOverview()).rejects.toThrow(/500/)
  })

  it('throws when a success response is somehow bodyless', async () => {
    get.mockResolvedValue({ data: undefined, error: undefined, response: res(200) })

    await expect(fetchOverview()).rejects.toThrow(/fetch overview/)
  })
})

describe('fetchFeedItems', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('asks for the given feed and returns its items', async () => {
    get.mockResolvedValue({ data: [makeItem()], response: res(200) })

    await expect(fetchFeedItems(7)).resolves.toHaveLength(1)
    expect(get).toHaveBeenCalledWith('/feeds/{feedId}/items', {
      params: { path: { feedId: 7 } },
    })
  })

  it('throws when the feed does not exist', async () => {
    get.mockResolvedValue({
      error: { code: 'not_found', message: 'the requested resource does not exist' },
      response: res(404),
    })

    await expect(fetchFeedItems(99)).rejects.toThrow(/not_found/)
  })

  it('throws — never resolves empty — when the response carries no usable body', async () => {
    get.mockResolvedValue({ data: undefined, error: undefined, response: res(500) })

    await expect(fetchFeedItems(1)).rejects.toThrow(/500/)
  })
})

describe('subscribeFeed', () => {
  beforeEach(() => {
    post.mockReset()
  })

  function madeFeed(): Feed {
    return {
      id: 7,
      url: 'https://a.test/feed.atom',
      title: 'GitHub',
      group_id: 0,
      enabled: true,
      refresh_interval_sec: 30,
      created_at: '2026-08-25T12:00:00Z',
      updated_at: '2026-08-25T12:00:00Z',
    }
  }

  // The cadence must reach the backend as seconds — the unit the contract, the database and
  // the poller all share, so nothing converts anything.
  it('posts the payload through unchanged, cadence in seconds', async () => {
    post.mockResolvedValue({ data: madeFeed(), response: res(201) })

    await subscribeFeed({
      title: 'GitHub',
      url: 'https://a.test/feed.atom',
      refresh_interval_sec: 30,
    })

    expect(post).toHaveBeenCalledWith('/feeds', {
      body: { title: 'GitHub', url: 'https://a.test/feed.atom', refresh_interval_sec: 30 },
    })
  })

  it('returns the created feed', async () => {
    post.mockResolvedValue({ data: madeFeed(), response: res(201) })

    const created = await subscribeFeed({
      title: 'GitHub',
      url: 'https://a.test/feed.atom',
      refresh_interval_sec: 300,
    })

    expect(created.id).toBe(7)
  })

  it('surfaces the server message so the banner can show it', async () => {
    post.mockResolvedValue({
      error: { code: 'name_exists', message: 'A system with that name already exists.' },
      response: res(409),
    })

    await expect(
      subscribeFeed({
        title: 'GitHub',
        url: 'https://a.test/feed.atom',
        refresh_interval_sec: 300,
      }),
    ).rejects.toMatchObject({
      status: 409,
      code: 'name_exists',
      serverMessage: 'A system with that name already exists.',
    })
  })

  it('throws — never pretends success — when the response carries no usable body', async () => {
    post.mockResolvedValue({ data: undefined, error: undefined, response: res(500) })

    await expect(
      subscribeFeed({
        title: 'GitHub',
        url: 'https://a.test/feed.atom',
        refresh_interval_sec: 300,
      }),
    ).rejects.toThrow(/500/)
  })
})
