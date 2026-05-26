<script setup lang="ts">
import { ref } from 'vue';
import Dialog from 'primevue/dialog';
import InputText from 'primevue/inputtext';
import Password from 'primevue/password';
import Button from 'primevue/button';
import Message from 'primevue/message';
import { useUserStore } from '@/stores/user.ts';
import { useI18nStore } from '@/stores/i18n.ts';

const props = defineProps<{
  visible: boolean;
}>();

const emit = defineEmits(['update:visible', 'login-success']);

const userStore = useUserStore();
const i18nStore = useI18nStore();
const username = ref('');
const password = ref('');
const error = ref('');
const loading = ref(false);

const handleLogin = async () => {
  if (!username.value || !password.value) {
    error.value = i18nStore.t('err_fields_required');
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
      error.value = i18nStore.t('err_invalid_credentials');
    }
  } catch (e) {
    error.value = i18nStore.t('err_login_failed');
  } finally {
    loading.value = false;
  }
};

const handleCancel = () => {
  emit('update:visible', false);
};
</script>

<template>
  <Dialog :visible="visible" @update:visible="emit('update:visible', $event)" modal :header="$t('login_header')" :style="{ width: '25rem' }">
    <div class="flex flex-col gap-4">
      <div class="flex flex-col gap-2">
        <label for="username">{{ $t('username') }}</label>
        <InputText id="username" v-model="username" @keyup.enter="handleLogin" autofocus />
      </div>
      <div class="flex flex-col gap-2">
        <label for="password">{{ $t('password') }}</label>
        <Password id="password" v-model="password" :feedback="false" toggleMask @keyup.enter="handleLogin" fluid />
      </div>
      <Message v-if="error" severity="error" variant="simple">{{ error }}</Message>
      <div class="flex justify-end gap-2">
        <Button type="button" :label="$t('cancel')" severity="secondary" @click="handleCancel"></Button>
        <Button type="button" :label="$t('login')" :loading="loading" @click="handleLogin"></Button>
      </div>
    </div>
  </Dialog>
</template>
