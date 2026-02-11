import {useUserStore} from "@/stores/user.ts";

// NOTE: Only set this to a KOsync Server when using vite dev
const BASE_URL = "";

export async function fetchApi<T>(route: string, options: RequestInit): Promise<{data: T | null, error: string | Response | null}> {
    const userStore = useUserStore();
    if (!userStore.isLoggedIn()) {
        return {data: null, error: "Not logged in"}
    }

    const response = await fetch(
      `${BASE_URL}${route}`,
      {
        ...options,
        headers: {...options.headers, "Authorization": `Bearer ${userStore.user.accessToken}`}
      }
    );
    if (!response.ok) return Promise.reject({data: null, error: response.statusText});

    if (response.headers.get('content-type')?.startsWith('application/json')) {
        const data = await response.json() as T;
        return {data, error: null}
    } else {
        return {data: await response.text() as T, error: null}
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
    return {data, error: null}
  } else {
    return {data: await response.text() as T, error: null}
  }
}

export async function openSocket(onConnected: (ws: WebSocket) => void, onMessage: (ws: WebSocket, message: string) => void) {
  const userStore = useUserStore();
  if (!userStore.isLoggedIn()) return;
  const ws = new WebSocket(`${BASE_URL}/api/ws/${userStore.user.accessToken}`, ["kosync.rpc", "kosync.pubsub"]);
  ws.onopen = (event) => onConnected(ws);
  ws.onmessage = (event) => onMessage(ws, event.data);
}
