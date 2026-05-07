<script setup lang="ts">
import DocumentsList from "@/components/DocumentsList.vue";
import LoginModal from "@/components/LoginModal.vue";
import {useUserStore} from "@/stores/user.ts";
import {useSyncStore} from "@/stores/sync.ts";
import {ref} from "vue";

const userStore = useUserStore();
const syncStore = useSyncStore();

const isLoggedIn = ref(false);
const loginVisible = ref(false);
isLoggedIn.value = await userStore.isLoggedIn();

const doLogin = async (token: string) => {
  const loginSuccess = await userStore.login(token);
  if (!loginSuccess) {
      alert("Failed to login, please check your credentials and try again.");
      return;
  }
  await onLoginSuccess();
}

const onLoginSuccess = async () => {
  isLoggedIn.value = await userStore.isLoggedIn();
  history.replaceState({}, document.title, document.location.pathname);
  await syncStore.doSync();
}

const uriParams = document.location.search;
if (uriParams) {
    const params = new URLSearchParams(uriParams);
    if (params.get("token") !== null) doLogin(params.get("token")!);
}

const openLogin = () => {
    loginVisible.value = true;
}

const doLogout = async () => {
  userStore.logout();
  syncStore.clear();
  isLoggedIn.value = false;
}
</script>

<template>
  <main class="m-4 flex flex-col gap-8">
    <div class="flex gap-2 justify-end">
      <Button v-if="!isLoggedIn" @click="openLogin">Login</Button>
      <Button v-if="isLoggedIn" variant="secondary" disabled>Logged in as '{{userStore.getUsername()}}'</Button>
      <Button v-if="isLoggedIn" @click="doLogout">Logout</Button>
    </div>
    <DocumentsList v-if="isLoggedIn" customTitle="My documents" />
    <LoginModal v-model:visible="loginVisible" @login-success="onLoginSuccess" />
  </main>
</template>
