import createClient from 'openapi-fetch'

import type { paths } from './schema.gen'

// Same origin in every environment: Vite proxies /api to the backend in development,
// and the single container serves both in production. Types come from the contract, so
// a spec change surfaces here as a type error rather than at runtime.
export const client = createClient<paths>({ baseUrl: '/api' })
