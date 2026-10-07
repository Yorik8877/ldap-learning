<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { apiService } from '@/common/services/api/api.service';
import type { Group, GroupSummary } from '@/common/services/api/models';
import { useNotifierStore } from '@/stores/notifier.store';
import GroupCreateDialog from '../components/GroupCreateDialog.vue';

const HEADERS = [
  { title: 'cn', key: 'name' },
  { title: 'description', key: 'description' },
  { title: 'Участников', key: 'memberCount' },
];

const notifier = useNotifierStore();
const router = useRouter();
const groups = ref<GroupSummary[]>([]);
const loading = ref(false);
const creating = ref(false);

async function load(): Promise<void> {
  loading.value = true;
  try {
    groups.value = await apiService.groups.list();
  } catch (caught) {
    notifier.error(caught);
  } finally {
    loading.value = false;
  }
}

function onRowClick(_event: Event, row: { item: GroupSummary }): void {
  void router.push({ name: 'group', params: { name: row.item.name } });
}

function onCreated(created: Group): void {
  notifier.success(`Группа ${created.name} создана`);
  void load();
}

onMounted(load);
</script>

<template>
  <v-container>
    <v-card title="Группы" subtitle="Записи groupOfUniqueNames в ou=groups">
      <template #append>
        <v-btn color="primary" prepend-icon="mdi-account-group" @click="creating = true">Создать</v-btn>
      </template>
      <v-data-table :headers="HEADERS" :items="groups" :loading="loading" item-value="name" hover @click:row="onRowClick" />
    </v-card>
    <GroupCreateDialog v-model="creating" @saved="onCreated" />
  </v-container>
</template>
