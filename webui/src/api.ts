import {useUserStore} from "@/stores/user.ts";

// NOTE: Only set this to a KOsync Server when using vite dev
const BASE_URL = "";

export async function fetchApi<T>(route: string, options: RequestInit): Promise<{data: T | null, error: string | Response | null}> {
    const userStore = useUserStore();
    if (!userStore.user.username || !userStore.user.key) {
        return Promise.resolve({data: null, error: "Not logged in"});
    }

    const response = await fetch(
      `${BASE_URL}${route}`,
      {
        ...options,
        headers: {...options.headers, 'x-auth-user': userStore.user.username, 'x-auth-key': userStore.user.key}
      }
    );
    if (!response.ok) return Promise.reject({data: null, error: response.statusText});

    if (response.headers.get('content-type')?.startsWith('application/json')) {
        const data = await response.json() as T;
        return Promise.resolve({data, error: null});
    } else {
        return Promise.resolve({data: await response.text() as T, error: null});
    }
}

export async function fetchUrl<T>(route: string, options: RequestInit): Promise<{data: T | null, error: string | Response | null}> {
  const response = await fetch(
    `${BASE_URL}${route}`,
    options
  );
  if (!response.ok) return Promise.reject({data: null, error: response});

  if (response.headers.get('content-type')?.startsWith('application/json')) {
    const data = await response.json() as T;
    return Promise.resolve({data, error: null});
  } else {
    return Promise.resolve({data: await response.text() as T, error: null});
  }
}
