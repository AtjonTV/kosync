import {type Ref, ref} from 'vue'
import { defineStore } from 'pinia'

export type UserState = {
  accessToken: string
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
    user.value = {accessToken: token}
    localStorage.setItem('userState', btoa(JSON.stringify(user.value)))
    return true;
  }

  function logout() {
    localStorage.removeItem('userState')
    user.value = {accessToken: ""}
  }

  function isLoggedIn(): boolean {
      return user.value.accessToken !== "";
  }

  return { user, login, logout, isLoggedIn }
})
