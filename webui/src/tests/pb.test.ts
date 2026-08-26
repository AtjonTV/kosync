//
// File:        webui/src/tests/pb.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fileUrl, pb } from '@/pb'

/**
 * A token the SDK will accept, expiring the given number of seconds from now.
 *
 * Neither signature nor payload is checked here — the client only reads "exp"
 * to decide whether what it holds is still worth sending.
 */
function token(id: string, seconds: number): string {
  const payload = { id, exp: Math.floor(Date.now() / 1000) + seconds }
  const encode = (value: object) => btoa(JSON.stringify(value)).replace(/=+$/, '')

  return `${encode({ alg: 'HS256' })}.${encode(payload)}.signature`
}

/** The book a cover is asked for. */
const book = { id: 'book-a', collectionId: 'pbc_books', collectionName: 'books' }

let issued: string[] = []
let refuse = false

function signIn(id = 'user-a'): void {
  pb.authStore.save(token(id, 3600), { id, collectionId: 'pbc_users', collectionName: 'users' })
}

/** Waits for the token request the render kicked off. */
async function settle(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, 0))
}

describe('file addresses', () => {
  beforeEach(() => {
    issued = []
    refuse = false
    let count = 0

    vi.stubGlobal('fetch', async () => {
      if (refuse) return new Response('{}', { status: 500 })

      count += 1
      const value = token('user-a', 3600) + `.${count}`
      issued.push(value)

      return new Response(JSON.stringify({ token: value }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      })
    })

    pb.authStore.clear()
  })

  afterEach(() => {
    pb.authStore.clear()
    vi.unstubAllGlobals()
  })

  it('renders no address until a token has arrived', async () => {
    signIn()

    expect(fileUrl(book, 'cover.jpg')).toBe('')

    await settle()

    expect(fileUrl(book, 'cover.jpg')).toContain(`token=${issued[0]}`)
  })

  it('asks for one token for a page full of covers', async () => {
    signIn()

    for (let i = 0; i < 20; i += 1) fileUrl(book, 'cover.jpg')
    await settle()

    expect(issued).toHaveLength(1)
  })

  it('reuses the token it holds', async () => {
    signIn()
    fileUrl(book, 'cover.jpg')
    await settle()

    const first = fileUrl(book, 'cover.jpg')
    await settle()

    expect(fileUrl(book, 'cover.jpg')).toBe(first)
    expect(issued).toHaveLength(1)
  })

  it('keeps the thumbnail size alongside the token', async () => {
    signIn()
    fileUrl(book, 'cover.jpg', '200x300')
    await settle()

    const address = fileUrl(book, 'cover.jpg', '200x300')

    expect(address).toContain('thumb=200x300')
    expect(address).toContain('token=')
  })

  it('asks for nothing while signed out', async () => {
    expect(fileUrl(book, 'cover.jpg')).toBe('')

    await settle()

    expect(issued).toHaveLength(0)
  })

  it('drops the token when another account signs in', async () => {
    signIn('user-a')
    fileUrl(book, 'cover.jpg')
    await settle()

    signIn('user-b')

    expect(fileUrl(book, 'cover.jpg')).toBe('')
  })

  it('renders no address for a field that holds no file', () => {
    signIn()

    expect(fileUrl(book, '')).toBe('')
  })

  it('does not ask again straight away after a refusal', async () => {
    refuse = true
    signIn()

    fileUrl(book, 'cover.jpg')
    await settle()

    refuse = false
    for (let i = 0; i < 5; i += 1) fileUrl(book, 'cover.jpg')
    await settle()

    expect(issued).toHaveLength(0)
  })
})
