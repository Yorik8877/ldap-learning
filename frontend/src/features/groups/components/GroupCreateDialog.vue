<script setup lang="ts">
import { ref, watch } from 'vue';
import { apiService } from '@/common/services/api/api.service';
import { errorMessage } from '@/common/services/api/api-error';
import type { Group, User } from '@/common/services/api/models';

const open = defineModel<boolean>({ required: true });
const emit = defineEmits<{ saved: [group: Group] }>();

const name = ref('');
const description = ref('');
const memberUids = ref<string[]>([]);
const users = ref<User[]>([]);
const errorText = ref('');
const saving = ref(false);

watch(open, async (isOpen) => {
  if (!isOpen) {
    return;
  }
  name.value = '';
  description.value = '';
  memberUids.value = [];
  errorText.value = '';
  try {
    users.value = await apiService.users.list();
  } catch (caught) {
    errorText.value = errorMessage(caught);
  }
});

async function save(): Promise<void> {
  saving.value = true;
  errorText.value = '';
  try {
    const created = await apiService.groups.create({
      name: name.value.trim(), description: description.value, memberUids: memberUids.value,
    });
    emit('saved', created);
    open.value = false;
  } catch (caught) {
    errorText.value = errorMessage(caught);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <v-dialog v-model="open" max-width="520">
    <v-card title="Новая группа" subtitle="groupOfUniqueNames требует хотя бы одного участника">
      <v-card-text>
        <v-text-field v-model="name" label="cn — имя группы" hint="Строчные латинские буквы, цифры, «.», «_», «-»" />
        <v-text-field v-model="description" label="description" />
        <v-autocomplete
          v-model="memberUids"
          :items="users"
          item-title="uid"
          item-value="uid"
          label="Участники (uniqueMember)"
          multiple
          chips
        />
        <v-alert v-if="errorText" type="error" variant="tonal">{{ errorText }}</v-alert>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn @click="open = false">Отмена</v-btn>
        <v-btn color="primary" :loading="saving" @click="save">Создать</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
