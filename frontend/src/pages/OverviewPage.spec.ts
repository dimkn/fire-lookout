import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { Feed, StatusItem, SystemOverview } from '@/api/status'
import { fetchFeedItems, fetchOverview, RequestError, subscribeFeed } from '@/api/status'
import AddIntegrationDialog from '@/components/AddIntegrationDialog/AddIntegrationDialog.vue'
import AppButton from '@/components/AppButton/AppButton.vue'
import SystemCard from '@/components/SystemCard/SystemCard.vue'
import PageToolbar from '@/components/PageToolbar/PageToolbar.vue'
import { clearBanners, currentBanner } from '@/composables/statusBanner'
import { MIN_LOADING_MS } from '@/utils/timing'

import OverviewPage from './OverviewPage.vue'

vi.mock('@/api/status', async (importOriginal) => {
  // RequestError is a real class the page checks with instanceof, so keep it genuine and
  // stub only the request functions.
  const actual = await importOriginal<typeof import('@/api/status')>()
  return {
    RequestError: actual.RequestError,
    fetchOverview: vi.fn(),
    fetchFeedItems: vi.fn(),
    subscribeFeed: vi.fn(),
  }
})

const fetchOverviewMock = vi.mocked(fetchOverview)
const fetchFeedItemsMock = vi.mocked(fetchFeedItems)
const subscribeFeedMock = vi.mocked(subscribeFeed)

function makeFeed(id: number, title: string): Feed {
  return {
    id,
    url: `https://${title.toLowerCase()}.test/feed.atom`,
    title,
    group_id: 0,
    enabled: true,
    refresh_interval_sec: 300,
    created_at: '2026-08-24T12:00:00Z',
    updated_at: '2026-08-24T12:00:00Z',
  }
}

function makeSystem(id: number, title: string): SystemOverview {
  return {
    feed: {
      id,
      url: `https://${title.toLowerCase()}.test/feed.atom`,
      title,
      group_id: 0,
      enabled: true,
      refresh_interval_sec: 300,
      last_fetched_at: '2026-08-18T09:05:00Z',
      last_success_at: '2026-08-18T09:05:00Z',
      created_at: '2026-08-01T00:00:00Z',
      updated_at: '2026-08-18T09:05:00Z',
    },
    indicator: 'operational',
    last_updated_at: '2026-08-18T09:00:00Z',
  }
}

function makeItem(id: number, feedId: number): StatusItem {
  return {
    id,
    feed_id: feedId,
    guid: `guid-${id}`,
    title: `Incident ${id}`,
    link: null,
    published_at: '2026-08-18T08:00:00Z',
    updated_at: null,
    content_html: null,
    content_text: 'Something happened.',
    status: 'resolved',
    fetched_at: '2026-08-18T09:05:00Z',
  }
}

async function mountPage() {
  const wrapper = mount(OverviewPage)
  await flushPromises()
  return wrapper
}

describe('OverviewPage', () => {
  beforeEach(() => {
    fetchOverviewMock.mockReset()
    fetchFeedItemsMock.mockReset()
    subscribeFeedMock.mockReset()
    fetchOverviewMock.mockResolvedValue([])
    fetchFeedItemsMock.mockResolvedValue([])
    subscribeFeedMock.mockResolvedValue(makeFeed(99, 'Something'))
    clearBanners()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('asks for the overview exactly once on load', async () => {
    await mountPage()

    expect(fetchOverviewMock).toHaveBeenCalledTimes(1)
    expect(fetchFeedItemsMock).not.toHaveBeenCalled()
  })

  it('renders a card per system', async () => {
    fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub'), makeSystem(2, 'Datadog')])

    const wrapper = await mountPage()

    expect(wrapper.findAllComponents(SystemCard)).toHaveLength(2)
  })

  it('leaves the area empty when nothing is saved yet', async () => {
    const wrapper = await mountPage()

    expect(wrapper.findAllComponents(SystemCard)).toHaveLength(0)
    expect(wrapper.get('.overview__systems').text()).toBe('')
  })

  it('fetches only the expanded system’s incidents', async () => {
    fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub'), makeSystem(2, 'Datadog')])
    fetchFeedItemsMock.mockResolvedValue([makeItem(201, 2)])

    const wrapper = await mountPage()
    await wrapper.findAll('.system-card__row')[1].trigger('click')
    await flushPromises()

    expect(fetchFeedItemsMock).toHaveBeenCalledTimes(1)
    expect(fetchFeedItemsMock).toHaveBeenCalledWith(2)
  })

  it('passes the fetched incidents back to the card that asked', async () => {
    fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub'), makeSystem(2, 'Datadog')])
    fetchFeedItemsMock.mockResolvedValue([makeItem(201, 2), makeItem(202, 2)])

    const wrapper = await mountPage()
    await wrapper.findAll('.system-card__row')[1].trigger('click')
    await flushPromises()

    const cards = wrapper.findAllComponents(SystemCard)
    expect(cards[1].props('items')).toHaveLength(2)
    expect(cards[0].props('items')).toEqual([])
  })

  it('keeps incidents it already has instead of refetching', async () => {
    fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub')])
    fetchFeedItemsMock.mockResolvedValue([makeItem(101, 1)])

    const wrapper = await mountPage()
    const row = wrapper.get('.system-card__row')
    await row.trigger('click')
    await flushPromises()
    await row.trigger('click')
    await row.trigger('click')
    await flushPromises()

    expect(fetchFeedItemsMock).toHaveBeenCalledTimes(1)
  })

  it('survives a failing overview request with an empty area', async () => {
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {})
    fetchOverviewMock.mockRejectedValue(new Error('fetch overview: 500'))

    const wrapper = await mountPage()

    expect(wrapper.findAllComponents(SystemCard)).toHaveLength(0)
    expect(logged).toHaveBeenCalled()
  })

  // An empty area alone cannot say whether the load failed or nothing is saved yet.
  it('announces a failed initial load', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    fetchOverviewMock.mockRejectedValue(new Error('fetch overview: HTTP 500'))

    await mountPage()

    expect(currentBanner.value).toMatchObject({
      kind: 'error',
      message: 'Could not load the status overview.',
    })
  })

  it('says nothing when the initial load succeeds', async () => {
    fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub')])

    await mountPage()

    expect(currentBanner.value).toBeNull()
  })

  it('names the system when its incidents cannot be loaded', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub')])
    fetchFeedItemsMock.mockRejectedValue(new Error('fetch items for feed 1: HTTP 500'))

    const wrapper = await mountPage()
    await wrapper.get('.system-card__row').trigger('click')
    await flushPromises()

    expect(currentBanner.value).toMatchObject({
      kind: 'error',
      message: 'Could not load incidents for GitHub.',
    })
  })

  it('survives a failing incident request with the card still open', async () => {
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {})
    fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub')])
    fetchFeedItemsMock.mockRejectedValue(new Error('fetch items for feed 1: 404'))

    const wrapper = await mountPage()
    await wrapper.get('.system-card__row').trigger('click')
    await flushPromises()

    expect(wrapper.find('.system-card__detail').exists()).toBe(true)
    expect(wrapper.getComponent(SystemCard).props('items')).toEqual([])
    expect(logged).toHaveBeenCalled()
  })

  describe('adding an integration', () => {
    beforeEach(() => {
      vi.useFakeTimers()
    })

    afterEach(() => {
      vi.useRealTimers()
    })

    async function openDialog() {
      fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub')])
      const wrapper = mount(OverviewPage, { attachTo: document.body })
      await flushPromises()
      wrapper.getComponent(PageToolbar).vm.$emit('add')
      await flushPromises()
      return wrapper
    }

    async function fillAndSave(wrapper: Awaited<ReturnType<typeof openDialog>>) {
      const inputs = wrapper.getComponent(AddIntegrationDialog).findAll('input')
      await inputs[0].setValue('Fastly')
      await inputs[1].setValue('https://fastly.test/feed.atom')
      await wrapper
        .getComponent(AddIntegrationDialog)
        .findAllComponents(AppButton)
        .find((b) => b.props('label') === 'Save')!
        .trigger('click')
    }

    async function settle() {
      await vi.advanceTimersByTimeAsync(MIN_LOADING_MS)
      await flushPromises()
    }

    it('shows no dialog until the button is pressed', async () => {
      fetchOverviewMock.mockResolvedValue([])

      const wrapper = await mountPage()

      expect(wrapper.findComponent(AddIntegrationDialog).exists()).toBe(false)
    })

    it('opens the dialog on the add button, without calling the API', async () => {
      const wrapper = await openDialog()

      expect(wrapper.findComponent(AddIntegrationDialog).exists()).toBe(true)
      expect(subscribeFeedMock).not.toHaveBeenCalled()
      expect(fetchOverviewMock).toHaveBeenCalledTimes(1)
      wrapper.unmount()
    })

    it('closes on Close, saying nothing', async () => {
      const wrapper = await openDialog()

      wrapper.getComponent(AddIntegrationDialog).vm.$emit('close')
      await flushPromises()

      expect(wrapper.findComponent(AddIntegrationDialog).exists()).toBe(false)
      expect(currentBanner.value).toBeNull()
      expect(subscribeFeedMock).not.toHaveBeenCalled()
      wrapper.unmount()
    })

    // Reopening must offer a blank form, never the abandoned draft.
    it('reopens empty after a cancelled draft', async () => {
      const wrapper = await openDialog()
      const inputs = wrapper.getComponent(AddIntegrationDialog).findAll('input')
      await inputs[0].setValue('Half-typed')

      wrapper.getComponent(AddIntegrationDialog).vm.$emit('close')
      await flushPromises()
      wrapper.getComponent(PageToolbar).vm.$emit('add')
      await flushPromises()

      const values = wrapper
        .getComponent(AddIntegrationDialog)
        .findAll('input')
        .map((i) => (i.element as HTMLInputElement).value)
      expect(values).toEqual(['', ''])
      wrapper.unmount()
    })

    it('sends what was typed', async () => {
      const wrapper = await openDialog()

      await fillAndSave(wrapper)
      await settle()

      expect(subscribeFeedMock).toHaveBeenCalledWith({
        title: 'Fastly',
        url: 'https://fastly.test/feed.atom',
      })
      wrapper.unmount()
    })

    it('marks the dialog as saving until the request settles', async () => {
      const wrapper = await openDialog()
      let release: (value: Feed) => void = () => {}
      subscribeFeedMock.mockReturnValueOnce(
        new Promise<Feed>((resolve) => {
          release = resolve
        }),
      )

      await fillAndSave(wrapper)
      expect(wrapper.getComponent(AddIntegrationDialog).props('saving')).toBe(true)

      release(makeFeed(2, 'Fastly'))
      await settle()

      expect(wrapper.findComponent(AddIntegrationDialog).exists()).toBe(false)
      wrapper.unmount()
    })

    it('closes the dialog, announces success and shows the new system', async () => {
      const wrapper = await openDialog()
      subscribeFeedMock.mockResolvedValueOnce(makeFeed(2, 'Fastly'))
      fetchOverviewMock.mockResolvedValueOnce([makeSystem(1, 'GitHub'), makeSystem(2, 'Fastly')])

      await fillAndSave(wrapper)
      await settle()

      expect(wrapper.findComponent(AddIntegrationDialog).exists()).toBe(false)
      expect(currentBanner.value).toMatchObject({ kind: 'success' })
      expect(currentBanner.value?.message).toContain('Fastly')
      // Re-read so the dashboard shows it without waiting for a manual refresh.
      expect(fetchOverviewMock).toHaveBeenCalledTimes(2)
      const names = wrapper.findAll('.system-card__name').map((n) => n.text())
      expect(names).toEqual(['GitHub', 'Fastly'])
      wrapper.unmount()
    })

    it('keeps the dialog open and shows the server’s message on failure', async () => {
      vi.spyOn(console, 'error').mockImplementation(() => {})
      const wrapper = await openDialog()
      subscribeFeedMock.mockRejectedValueOnce(
        new RequestError(
          409,
          'name_exists',
          'A system with that name already exists.',
          'add integration failed',
        ),
      )

      await fillAndSave(wrapper)
      await settle()

      expect(wrapper.findComponent(AddIntegrationDialog).exists()).toBe(true)
      expect(currentBanner.value).toMatchObject({
        kind: 'error',
        message: 'A system with that name already exists.',
      })
      // The list must not be re-read when nothing was created.
      expect(fetchOverviewMock).toHaveBeenCalledTimes(1)
      wrapper.unmount()
    })

    it('falls back to its own wording when the failure carries no message', async () => {
      vi.spyOn(console, 'error').mockImplementation(() => {})
      const wrapper = await openDialog()
      subscribeFeedMock.mockRejectedValueOnce(new Error('network down'))

      await fillAndSave(wrapper)
      await settle()

      expect(currentBanner.value?.kind).toBe('error')
      expect(currentBanner.value?.message).not.toContain('network down')
      expect(currentBanner.value?.message.length).toBeGreaterThan(0)
      wrapper.unmount()
    })

    it('lets the dialog be retried after a failure', async () => {
      vi.spyOn(console, 'error').mockImplementation(() => {})
      const wrapper = await openDialog()
      subscribeFeedMock.mockRejectedValueOnce(
        new RequestError(422, 'feed_unreachable', 'Could not read a feed there.', 'failed'),
      )

      await fillAndSave(wrapper)
      await settle()
      expect(wrapper.getComponent(AddIntegrationDialog).props('saving')).toBe(false)

      subscribeFeedMock.mockResolvedValueOnce(makeFeed(2, 'Fastly'))
      fetchOverviewMock.mockResolvedValueOnce([makeSystem(2, 'Fastly')])
      await wrapper
        .getComponent(AddIntegrationDialog)
        .findAllComponents(AppButton)
        .find((b) => b.props('label') === 'Save')!
        .trigger('click')
      await settle()

      expect(subscribeFeedMock).toHaveBeenCalledTimes(2)
      expect(wrapper.findComponent(AddIntegrationDialog).exists()).toBe(false)
      wrapper.unmount()
    })
  })

  describe('refresh', () => {
    // Refresh holds its loading state for a minimum time, so these tests drive the clock
    // instead of waiting on it.
    beforeEach(() => {
      vi.useFakeTimers()
    })

    afterEach(() => {
      vi.useRealTimers()
    })

    async function clickRefresh(wrapper: Awaited<ReturnType<typeof mountPage>>) {
      await wrapper.get('.app-button').trigger('click')
    }

    /** Let the request resolve and the minimum-duration floor elapse. */
    async function settleRefresh() {
      await vi.advanceTimersByTimeAsync(MIN_LOADING_MS)
      await flushPromises()
    }

    /** A request the test releases by hand, to model a slow API. */
    function gateNextRequest() {
      let release: (value: SystemOverview[]) => void = () => {}
      fetchOverviewMock.mockReturnValueOnce(
        new Promise<SystemOverview[]>((resolve) => {
          release = resolve
        }),
      )
      return (systems: SystemOverview[] = []) => release(systems)
    }

    it('re-reads the overview and nothing else', async () => {
      fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub')])

      const wrapper = await mountPage()
      await clickRefresh(wrapper)
      await settleRefresh()

      expect(fetchOverviewMock).toHaveBeenCalledTimes(2)
      expect(fetchFeedItemsMock).not.toHaveBeenCalled()
    })

    it('replaces the whole list with whatever the API returns', async () => {
      fetchOverviewMock.mockResolvedValueOnce([makeSystem(1, 'GitHub')])
      fetchOverviewMock.mockResolvedValueOnce([makeSystem(2, 'Datadog'), makeSystem(3, 'Fastly')])

      const wrapper = await mountPage()
      expect(wrapper.findAllComponents(SystemCard)).toHaveLength(1)

      await clickRefresh(wrapper)
      await settleRefresh()

      const names = wrapper.findAll('.system-card__name').map((node) => node.text())
      expect(names).toEqual(['Datadog', 'Fastly'])
    })

    it('leaves the visible data untouched when the request fails', async () => {
      const logged = vi.spyOn(console, 'error').mockImplementation(() => {})
      fetchOverviewMock.mockResolvedValueOnce([makeSystem(1, 'GitHub')])
      fetchOverviewMock.mockRejectedValueOnce(new Error('fetch overview: 500'))

      const wrapper = await mountPage()
      await clickRefresh(wrapper)
      await settleRefresh()

      const names = wrapper.findAll('.system-card__name').map((node) => node.text())
      expect(names).toEqual(['GitHub'])
      expect(logged).toHaveBeenCalled()
    })

    it('announces a completed refresh', async () => {
      fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub')])

      const wrapper = await mountPage()
      await clickRefresh(wrapper)
      await settleRefresh()

      expect(currentBanner.value).toMatchObject({ kind: 'success', message: 'Status updated.' })
    })

    // The data is stale but correct, and the message says so rather than implying loss.
    it('announces a failed refresh and says the old data still stands', async () => {
      vi.spyOn(console, 'error').mockImplementation(() => {})
      fetchOverviewMock.mockResolvedValueOnce([makeSystem(1, 'GitHub')])
      fetchOverviewMock.mockRejectedValueOnce(new Error('fetch overview: HTTP 500'))

      const wrapper = await mountPage()
      await clickRefresh(wrapper)
      await settleRefresh()

      expect(currentBanner.value).toMatchObject({
        kind: 'error',
        message: 'Could not refresh. Showing the last known status.',
      })
    })

    it('keeps already-fetched incidents, since only the overview is re-read', async () => {
      fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub')])
      fetchFeedItemsMock.mockResolvedValue([makeItem(101, 1)])

      const wrapper = await mountPage()
      await wrapper.get('.system-card__row').trigger('click')
      await flushPromises()

      await clickRefresh(wrapper)
      await settleRefresh()

      expect(wrapper.getComponent(SystemCard).props('items')).toHaveLength(1)
      expect(fetchFeedItemsMock).toHaveBeenCalledTimes(1)
    })

    it('marks the refresh button as loading until the request settles', async () => {
      fetchOverviewMock.mockResolvedValueOnce([makeSystem(1, 'GitHub')])
      const release = gateNextRequest()

      const wrapper = await mountPage()
      const toolbar = wrapper.getComponent(PageToolbar)
      expect(toolbar.props('refreshing')).toBe(false)

      await clickRefresh(wrapper)
      expect(toolbar.props('refreshing')).toBe(true)

      release([makeSystem(2, 'Datadog')])
      await settleRefresh()

      expect(toolbar.props('refreshing')).toBe(false)
    })

    it('ignores further presses while a refresh is in flight', async () => {
      fetchOverviewMock.mockResolvedValueOnce([makeSystem(1, 'GitHub')])
      const release = gateNextRequest()

      const wrapper = await mountPage()
      await clickRefresh(wrapper)
      await clickRefresh(wrapper)
      await clickRefresh(wrapper)

      expect(fetchOverviewMock).toHaveBeenCalledTimes(2) // the mount load plus one refresh

      release([])
      await settleRefresh()
    })

    it('clears the loading state even when the request fails', async () => {
      vi.spyOn(console, 'error').mockImplementation(() => {})
      fetchOverviewMock.mockResolvedValueOnce([makeSystem(1, 'GitHub')])
      fetchOverviewMock.mockRejectedValueOnce(new Error('fetch overview: 500'))

      const wrapper = await mountPage()
      await clickRefresh(wrapper)
      await settleRefresh()

      expect(wrapper.getComponent(PageToolbar).props('refreshing')).toBe(false)
    })

    describe('minimum duration', () => {
      // A local backend answers in single-digit milliseconds, which would flash the loader
      // for a frame or two. The floor makes the state legible.
      it('keeps the loader up for the full minimum when the API answers instantly', async () => {
        fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub')])

        const wrapper = await mountPage()
        const toolbar = wrapper.getComponent(PageToolbar)

        await clickRefresh(wrapper)
        await flushPromises() // the request itself has already resolved
        expect(toolbar.props('refreshing')).toBe(true)

        await vi.advanceTimersByTimeAsync(MIN_LOADING_MS - 1)
        expect(toolbar.props('refreshing')).toBe(true)

        await vi.advanceTimersByTimeAsync(1)
        expect(toolbar.props('refreshing')).toBe(false)
      })

      // The new list and the idle button appear together: no frame shows fresh data under
      // a spinning loader.
      it('reveals the new data when the loader goes away, not before', async () => {
        fetchOverviewMock.mockResolvedValueOnce([makeSystem(1, 'GitHub')])
        fetchOverviewMock.mockResolvedValueOnce([makeSystem(2, 'Datadog')])

        const wrapper = await mountPage()
        const names = () => wrapper.findAll('.system-card__name').map((node) => node.text())

        await clickRefresh(wrapper)
        await flushPromises()
        expect(names()).toEqual(['GitHub'])

        await vi.advanceTimersByTimeAsync(MIN_LOADING_MS)
        expect(names()).toEqual(['Datadog'])
        expect(wrapper.getComponent(PageToolbar).props('refreshing')).toBe(false)
      })

      it('holds the minimum on failure too, so a fast error does not flicker', async () => {
        vi.spyOn(console, 'error').mockImplementation(() => {})
        fetchOverviewMock.mockResolvedValueOnce([makeSystem(1, 'GitHub')])
        fetchOverviewMock.mockRejectedValueOnce(new Error('fetch overview: HTTP 500'))

        const wrapper = await mountPage()
        const toolbar = wrapper.getComponent(PageToolbar)

        await clickRefresh(wrapper)
        await flushPromises()
        expect(toolbar.props('refreshing')).toBe(true)

        await vi.advanceTimersByTimeAsync(MIN_LOADING_MS)
        expect(toolbar.props('refreshing')).toBe(false)
        expect(wrapper.findAll('.system-card__name').map((n) => n.text())).toEqual(['GitHub'])
      })

      // The floor is a minimum, not a timeout: a slow request still owns the loader.
      it('waits for a slow request beyond the minimum', async () => {
        fetchOverviewMock.mockResolvedValueOnce([makeSystem(1, 'GitHub')])
        const release = gateNextRequest()

        const wrapper = await mountPage()
        const toolbar = wrapper.getComponent(PageToolbar)

        await clickRefresh(wrapper)
        await vi.advanceTimersByTimeAsync(MIN_LOADING_MS * 4)
        expect(toolbar.props('refreshing')).toBe(true)

        release([makeSystem(2, 'Datadog')])
        await flushPromises()

        expect(toolbar.props('refreshing')).toBe(false)
        expect(wrapper.findAll('.system-card__name').map((n) => n.text())).toEqual(['Datadog'])
      })
    })

    // The initial page load deliberately has no loading state; the loader is a Refresh
    // affordance only.
    it('does not show the loader during the initial load', async () => {
      let release: (value: SystemOverview[]) => void = () => {}
      fetchOverviewMock.mockReturnValueOnce(
        new Promise<SystemOverview[]>((resolve) => {
          release = resolve
        }),
      )

      const wrapper = mount(OverviewPage)

      expect(wrapper.getComponent(PageToolbar).props('refreshing')).toBe(false)

      release([])
      await flushPromises()
    })
  })
})
