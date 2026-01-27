import { ref, computed } from 'vue'
import { defineStore } from 'pinia'

export const useUserStore = defineStore('user', () => {
  const userStateEncoded = localStorage.getItem('userState')
  const userState = userStateEncoded !== null ? JSON.parse(atob(userStateEncoded)) : null

  const user = ref(userState ?? {
    username: "",
    key: ""
  })

  async function login(username: string, key: string): Promise<boolean> {
    user.value = {username, key}
    localStorage.setItem('userState', btoa(JSON.stringify(user.value)))
    return true;
  }

  function logout() {
    localStorage.removeItem('userState')
    user.value = {username: "", key: ""}
  }

  function isLoggedIn(): boolean {
      return user.value.username !== "" && user.value.key !== "";
  }

  return { user, login, logout, isLoggedIn }
})
