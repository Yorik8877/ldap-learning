<script setup lang="ts">
import { useRouter } from 'vue-router';
import { useAuthStore } from '@/features/auth';
import { useNotifierStore } from '@/stores/notifier.store';

interface NavigationItem {
  title: string;
  icon: string;
  routeName: string;
}

const NAVIGATION: NavigationItem[] = [
  { title: 'Главная', icon: 'mdi-home', routeName: 'home' },
];

const auth = useAuthStore();
const notifier = useNotifierStore();
const router = useRouter();

async function signOut(): Promise<void> {
  try {
    await auth.signOut();
  } catch (caught) {
    notifier.error(caught);
  }
  await router.push({ name: 'login' });
}
</script>

<template>
  <v-app-bar density="compact" color="primary">
    <v-app-bar-title>LDAP Admin</v-app-bar-title>
    <v-btn
      v-for="item in NAVIGATION"
      :key="item.routeName"
      :to="{ name: item.routeName }"
      :prepend-icon="item.icon"
      variant="text"
    >
      {{ item.title }}
    </v-btn>
    <v-spacer />
    <span class="mr-2">{{ auth.user?.commonName }}</span>
    <v-btn icon="mdi-logout" title="Выйти" @click="signOut" />
  </v-app-bar>
  <v-main>
    <router-view />
  </v-main>
</template>
