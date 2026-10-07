import { createPinia } from 'pinia';
import { createApp } from 'vue';
import App from './App.vue';
import { setUnauthorizedHandler } from '@/common/services/api/http-client';
import { useAuthStore } from '@/features/auth';
import { vuetify } from './libs/vuetify';
import { router } from './router';

const app = createApp(App);
app.use(createPinia());
app.use(router);
app.use(vuetify);

// Сессия истекла посреди работы — забываем пользователя и отправляем на вход.
setUnauthorizedHandler(() => {
  useAuthStore().reset();
  const current = router.currentRoute.value;
  if (!current.meta.publicAccess) {
    void router.push({ name: 'login', query: { redirect: current.fullPath } });
  }
});

app.mount('#app');
