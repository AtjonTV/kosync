<script setup lang="ts">
import DocumentsList from "@/components/DocumentsList.vue";
import {useUserStore} from "@/stores/user.ts";
import {useSyncStore} from "@/stores/sync.ts";

const userStore = useUserStore();
const syncStore = useSyncStore();

const doLogin = async (userData: string) => {
    const usrObj = JSON.parse(atob(userData));
    const loginSuccess = await userStore.login(usrObj.username, usrObj.key);
    if (!loginSuccess) {
        alert("Failed to login, please check your credentials and try again.");
        return;
    }
    document.location.search = "";
    await syncStore.doSync();
}

const uriParams = document.location.search;
if (uriParams) {
    const params = new URLSearchParams(uriParams);
    if (params.get("user") !== null) doLogin(params.get("user")!);
}

const doLoginRedir = () => {
    location.replace("/api/auth.basic?redirect=" + encodeURIComponent(location.href));
}

const doLogout = async () => {
  userStore.logout();
  syncStore.clear();
}
</script>

<template>
  <main class="m-4 flex flex-col gap-8">
    <div class="flex gap-2 justify-end">
      <Button v-if="!userStore.isLoggedIn()" @click="doLoginRedir">Login</Button>
      <Button v-if="userStore.isLoggedIn()" variant="secondary" disabled>Logged in as '{{userStore.user.username}}'</Button>
      <Button v-if="userStore.isLoggedIn()" @click="doLogout">Logout</Button>
    </div>
    <DocumentsList customTitle="My documents" />
  </main>
</template>
