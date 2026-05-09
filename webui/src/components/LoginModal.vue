<script setup lang="ts">
import { ref } from 'vue';
import Dialog from 'primevue/dialog';
import InputText from 'primevue/inputtext';
import Password from 'primevue/password';
import Button from 'primevue/button';
import Message from 'primevue/message';
import { useUserStore } from '@/stores/user.ts';

const props = defineProps<{
  visible: boolean;
}>();

const emit = defineEmits(['update:visible', 'login-success']);

const userStore = useUserStore();
const username = ref('');
const password = ref('');
const error = ref('');
const loading = ref(false);

const handleLogin = async () => {
  if (!username.value || !password.value) {
    error.value = 'Please enter both username and password.';
    return;
  }

  loading.value = true;
  error.value = '';

  try {
    const success = await userStore.loginWithCredentials(username.value, password.value);
    if (success) {
      emit('login-success');
      emit('update:visible', false);
      username.value = '';
      password.value = '';
    } else {
      error.value = 'Invalid username or password.';
    }
  } catch (e) {
    error.value = 'An error occurred during login.';
  } finally {
    loading.value = false;
  }
};

const handleCancel = () => {
  emit('update:visible', false);
};
</script>

<template>
  <Dialog :visible="visible" @update:visible="emit('update:visible', $event)" modal header="Login" :style="{ width: '25rem' }">
    <div class="flex flex-col gap-4">
      <div class="flex flex-col gap-2">
        <label for="username">Username</label>
        <InputText id="username" v-model="username" @keyup.enter="handleLogin" autofocus />
      </div>
      <div class="flex flex-col gap-2">
        <label for="password">Password</label>
        <Password id="password" v-model="password" :feedback="false" toggleMask @keyup.enter="handleLogin" fluid />
      </div>
      <Message v-if="error" severity="error" variant="simple">{{ error }}</Message>
      <div class="flex justify-end gap-2">
        <Button type="button" label="Cancel" severity="secondary" @click="handleCancel"></Button>
        <Button type="button" label="Login" :loading="loading" @click="handleLogin"></Button>
      </div>
    </div>
  </Dialog>
</template>
