//
// File:        webui/src/main.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import './assets/main.css'
import 'primeicons/primeicons.css'

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config';
import Aura from '@primeuix/themes/aura';
import { definePreset } from '@primeuix/themes';
import ConfirmationService from 'primevue/confirmationservice';
import ToastService from 'primevue/toastservice';

import App from './App.vue'
import router from './router'

const app = createApp(App)

const KOsyncPreset = definePreset(Aura, {
    semantic: {
        primary: {
            50: '{blue.50}',
            100: '{blue.100}',
            200: '{blue.200}',
            300: '{blue.300}',
            400: '{blue.400}',
            500: '{blue.500}',
            600: '{blue.600}',
            700: '{blue.700}',
            800: '{blue.800}',
            900: '{blue.900}',
            950: '{blue.950}'
        }
    }
});

const pinia = createPinia()
app.use(pinia)

import { useI18nStore } from '@/stores/i18n.ts'
const i18nStore = useI18nStore()
app.config.globalProperties.$t = (key: string, ...args: any[]) => i18nStore.t(key, ...args)

app.use(router)
app.use(PrimeVue, {
    theme: {
        preset: KOsyncPreset,
        options: {
            darkModeSelector: '.p-dark',
        }
    }
});
app.use(ConfirmationService);
app.use(ToastService);

app.mount('#app')
