import {type Ref, ref} from 'vue'
import { defineStore } from 'pinia'
import {fetchApi} from "@/api.ts";
import md5 from 'blueimp-md5';

export type UserState = {
  accessToken: string,
  lastCheck: number,
}

export const useUserStore = defineStore('user', () => {
  const userStateEncoded = localStorage.getItem('userState')
  let userState = userStateEncoded === null ? null : JSON.parse(atob(userStateEncoded))

  // Because we only have "accessToken" now, force unset so users signin again
  if (userState && (userState.username || userState.password)) {
    userState = null;
  }

  const user: Ref<UserState> = ref(userState ?? {
    accessToken: ""
  })

  async function login(token: string): Promise<boolean> {
    user.value = {accessToken: token, lastCheck: 0}
    localStorage.setItem('userState', btoa(JSON.stringify(user.value)))
    return true;
  }

  async function loginWithCredentials(username: string, password: string): Promise<boolean> {
    const response = await fetch("/api/auth.jwt", {
      method: "GET",
      headers: {
        "x-auth-user": username,
        "x-auth-key": md5(password)
      }
    });

    if (!response.ok) {
      return false;
    }

    const token = await response.text();
    return await login(token);
  }

  function logout() {
    localStorage.removeItem('userState')
    user.value = {accessToken: "", lastCheck: 0}
  }

  async function isLoggedIn(): Promise<boolean> {
    if (!hasToken()) return false;
    if (Date.now() - user.value.lastCheck < 60_000) return true;

    const {data, error} = await fetchApi<string>("/users/auth");
    if (error && error === "Unauthorized") {
      logout();
      return false;
    } else if (error) {
      return false;
    } else {
      user.value.lastCheck = Date.now();
      localStorage.setItem('userState', btoa(JSON.stringify(user.value)))
      return data !== null && data === "OK";
    }
  }

  function hasToken(): boolean {
    //bearer:disable javascript_lang_observable_timing
    return user.value.accessToken !== "";
  }

  function getUsername(): string|null {
    if (!hasToken()) return null;
    const parts = user.value.accessToken.split('.');
    if (parts.length < 2 || !parts[1]) return null;
    const claims = JSON.parse(atob(parts[1]));
    return claims.username ?? null;
  }

  return { user, login, loginWithCredentials, logout, isLoggedIn, hasToken, getUsername }
})
