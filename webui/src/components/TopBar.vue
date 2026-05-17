//
// File:        src/components/TopBar.vue
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//
<script setup lang="ts">
import { ref } from "vue";
import { useUserStore } from "@/stores/user.ts";
import { useSyncStore } from "@/stores/sync.ts";
import LoginModal from "@/components/LoginModal.vue";

const userStore = useUserStore();
const syncStore = useSyncStore();

const isLoggedIn = ref(false);
const loginVisible = ref(false);

const emit = defineEmits(['login-success', 'logout']);

userStore.isLoggedIn().then(status => {
    isLoggedIn.value = status;
});

const openLogin = () => {
    loginVisible.value = true;
};

const doLogout = async () => {
    userStore.logout();
    syncStore.clear();
    isLoggedIn.value = false;
    emit('logout');
};

const onLoginSuccess = async () => {
    isLoggedIn.value = await userStore.isLoggedIn();
    history.replaceState({}, document.title, document.location.pathname);
    await syncStore.doSync();
    emit('login-success');
};

const doLogin = async (token: string) => {
    const loginSuccess = await userStore.login(token);
    if (!loginSuccess) {
        alert("Failed to login, please check your credentials and try again.");
        return;
    }
    await onLoginSuccess();
};

const uriParams = document.location.search;
if (uriParams) {
    const params = new URLSearchParams(uriParams);
    if (params.get("token") !== null) doLogin(params.get("token")!);
}

const isDarkMode = ref(false);

const initTheme = () => {
    const savedTheme = localStorage.getItem('theme');
    if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
        document.documentElement.classList.add('p-dark');
        isDarkMode.value = true;
    } else {
        document.documentElement.classList.remove('p-dark');
        isDarkMode.value = false;
    }
};

initTheme();

const toggleTheme = () => {
    const root = document.documentElement;
    if (root.classList.contains('p-dark')) {
        root.classList.remove('p-dark');
        localStorage.setItem('theme', 'light');
        isDarkMode.value = false;
    } else {
        root.classList.add('p-dark');
        localStorage.setItem('theme', 'dark');
        isDarkMode.value = true;
    }
};

defineExpose({ openLogin });
</script>

<template>
  <div class="flex justify-between items-center px-6 py-4 border-b border-surface-200 dark:border-surface-700 bg-surface-0 dark:bg-surface-900">
    <div class="flex items-center gap-2">
      <h1 class="text-2xl font-bold">KOsync</h1>
    </div>
    <div class="flex items-center gap-3">
      <Button :icon="isDarkMode ? 'pi pi-sun' : 'pi pi-moon'" variant="text" rounded @click="toggleTheme" title="Toggle Theme" />
      <Button v-if="!isLoggedIn" @click="openLogin">Login</Button>
      <Button v-if="isLoggedIn" variant="secondary" disabled>
        <i class="pi pi-user mr-2"></i>
        <span class="hidden sm:inline">Logged in as&nbsp;</span>
        <span class="font-bold">{{userStore.getUsername()}}</span>
      </Button>
      <Button v-if="isLoggedIn" @click="doLogout">Logout</Button>
    </div>
    <LoginModal v-model:visible="loginVisible" @login-success="onLoginSuccess" />
  </div>
</template>
