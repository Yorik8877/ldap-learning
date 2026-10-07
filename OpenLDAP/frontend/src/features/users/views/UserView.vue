<script setup lang="ts">
import { ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { apiService } from '@/common/services/api/api.service';
import type { UserDetails } from '@/common/services/api/models';
import { useNotifierStore } from '@/stores/notifier.store';
import PasswordDialog from '../components/PasswordDialog.vue';
import UserFormDialog from '../components/UserFormDialog.vue';

const props = defineProps<{ uid: string }>();

const notifier = useNotifierStore();
const router = useRouter();
const details = ref<UserDetails | null>(null);
const editing = ref(false);
const changingPassword = ref(false);
const confirmingDelete = ref(false);
const deleting = ref(false);

async function load(): Promise<void> {
  try {
    details.value = await apiService.users.get(props.uid);
  } catch (caught) {
    notifier.error(caught);
  }
}

function onSaved(): void {
  notifier.success('Изменения сохранены');
  void load();
}

async function remove(): Promise<void> {
  deleting.value = true;
  try {
    await apiService.users.remove(props.uid);
    notifier.success(`Пользователь ${props.uid} удалён`);
    await router.push({ name: 'users' });
  } catch (caught) {
    notifier.error(caught);
  } finally {
    deleting.value = false;
    confirmingDelete.value = false;
  }
}

watch(() => props.uid, load, { immediate: true });
</script>

<template>
  <v-container>
    <v-card v-if="details" :title="details.commonName" :subtitle="`uid=${details.uid},ou=people`">
      <v-card-text>
        <v-list density="compact">
          <v-list-item title="sn" :subtitle="details.surname" />
          <v-list-item title="mail" :subtitle="details.emails.join(', ') || '—'" />
        </v-list>
        <div class="mt-4">Группы — из служебного атрибута memberOf, его ведёт сервер:</div>
        <v-chip v-for="name in details.groups" :key="name" :to="{ name: 'group', params: { name } }" class="mr-2 mt-2">{{ name }}</v-chip>
        <span v-if="details.groups.length === 0">нет</span>
      </v-card-text>
      <v-card-actions>
        <v-btn prepend-icon="mdi-pencil" @click="editing = true">Редактировать</v-btn>
        <v-btn prepend-icon="mdi-key" @click="changingPassword = true">Сменить пароль</v-btn>
        <v-spacer />
        <v-btn color="error" prepend-icon="mdi-delete" @click="confirmingDelete = true">Удалить</v-btn>
      </v-card-actions>
    </v-card>
    <UserFormDialog v-model="editing" :user="details" @saved="onSaved" />
    <PasswordDialog v-model="changingPassword" :uid="uid" @saved="notifier.success('Пароль изменён')" />
    <ConfirmDialog
      v-model="confirmingDelete"
      title="Удалить пользователя?"
      :text="`Запись uid=${uid} будет удалена. Из групп её уберёт сам сервер (оверлей refint).`"
      :loading="deleting"
      @confirm="remove"
    />
  </v-container>
</template>
