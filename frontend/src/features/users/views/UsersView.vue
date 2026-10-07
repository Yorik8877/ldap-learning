<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { apiService } from '@/common/services/api/api.service';
import type { User } from '@/common/services/api/models';
import { useNotifierStore } from '@/stores/notifier.store';
import UserFormDialog from '../components/UserFormDialog.vue';

const HEADERS = [
  { title: 'uid', key: 'uid' },
  { title: 'cn', key: 'commonName' },
  { title: 'sn', key: 'surname' },
  { title: 'mail', key: 'emails' },
];

const notifier = useNotifierStore();
const router = useRouter();
const users = ref<User[]>([]);
const loading = ref(false);
const creating = ref(false);

async function load(): Promise<void> {
  loading.value = true;
  try {
    users.value = await apiService.users.list();
  } catch (caught) {
    notifier.error(caught);
  } finally {
    loading.value = false;
  }
}

function onRowClick(_event: Event, row: { item: User }): void {
  void router.push({ name: 'user', params: { uid: row.item.uid } });
}

function onCreated(created: User): void {
  notifier.success(`Пользователь ${created.uid} создан`);
  void load();
}

onMounted(load);
</script>

<template>
  <v-container>
    <v-card title="Пользователи" subtitle="Записи inetOrgPerson в ou=people">
      <template #append>
        <v-btn color="primary" prepend-icon="mdi-account-plus" @click="creating = true">Создать</v-btn>
      </template>
      <v-data-table :headers="HEADERS" :items="users" :loading="loading" item-value="uid" hover @click:row="onRowClick">
        <template #[`item.emails`]="{ value }">{{ value.join(', ') }}</template>
      </v-data-table>
    </v-card>
    <UserFormDialog v-model="creating" :user="null" @saved="onCreated" />
  </v-container>
</template>
