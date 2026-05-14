//
// File:        webui/src/tests/setup.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { setActivePinia, createPinia } from 'pinia'
import { vi, beforeEach, afterEach } from "vitest";

// Global test setup: reset Pinia and storage before each test
beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  setActivePinia(createPinia());
  // Provide a default fetch stub so relative URLs don't throw ERR_INVALID_URL.
  // Individual tests can override this with vi.stubGlobal('fetch', ...).
  if (!(globalThis.fetch as any)?._isMockFunction) {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      statusText: 'Not Found',
      headers: { get: () => null },
      text: async () => '',
      json: async () => null,
    }));
  }
});

afterEach(() => {
  vi.unstubAllGlobals();
});
