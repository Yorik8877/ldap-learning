<script setup lang="ts">
import { ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { loginErrorMessage } from '../helpers/login-error';
import { safeRedirectTarget } from '../helpers/redirect-target';
import { useAuthStore } from '../stores/auth.store';

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();

const uid = ref('');
const password = ref('');
const errorText = ref('');
const submitting = ref(false);

async function submit(): Promise<void> {
  errorText.value = '';
  submitting.value = true;
  try {
    await auth.signIn(uid.value.trim(), password.value);
    await router.replace(safeRedirectTarget(route.query.redirect));
  } catch (caught) {
    errorText.value = loginErrorMessage(caught);
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <v-main>
    <v-container class="fill-height d-flex justify-center">
      <v-card width="420" title="LDAP Admin" subtitle="Вход для участников группы admins">
        <v-card-text>
          <v-form @submit.prevent="submit">
            <v-text-field v-model="uid" label="uid" autocomplete="username" autofocus />
            <v-text-field v-model="password" label="Пароль" type="password" autocomplete="current-password" />
            <v-alert v-if="errorText" type="error" variant="tonal" class="mb-4">{{ errorText }}</v-alert>
            <v-btn type="submit" color="primary" block :loading="submitting">Войти</v-btn>
          </v-form>
        </v-card-text>
      </v-card>
    </v-container>
  </v-main>
</template>
