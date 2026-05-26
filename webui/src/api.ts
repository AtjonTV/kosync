import {useUserStore} from "@/stores/user.ts";
import {useI18nStore} from "@/stores/i18n.ts";

// NOTE: Only set this to a KOsync Server when using vite dev
const BASE_URL = "";

export async function fetchApi<T>(route: string, options: RequestInit = {}): Promise<{data: T | null, error: string | Response | null}> {
    const userStore = useUserStore();
    if (!userStore.hasToken()) {
        return {data: null, error: "Not logged in"}
    }

    const i18nStore = useI18nStore();

    const response = await fetch(
      `${BASE_URL}${route}`,
      {
        ...options,
        headers: {
          ...options.headers,
          "Authorization": `Bearer ${userStore.user.accessToken}`,
          "Accept-Language": i18nStore.locale
        }
      }
    );
    if (!response.ok) return {data: null, error: response.statusText};

    if (response.headers.get('content-type')?.startsWith('application/json')) {
        const data = await response.json() as T;
        return {data, error: null}
    } else {
        return {data: await response.text() as T, error: null}
    }
}

export function getWebSocketUrl(): string|null {
  const userStore = useUserStore();
  if (!userStore.isLoggedIn()) return null;
  const i18nStore = useI18nStore();
  return `${BASE_URL}/api/ws/${userStore.user.accessToken}?lang=${i18nStore.locale}`;
}
