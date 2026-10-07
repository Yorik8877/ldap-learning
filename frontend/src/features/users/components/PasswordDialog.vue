<script setup lang="ts">
import { ref, watch } from 'vue';
import { apiService } from '@/common/services/api/api.service';
import { errorMessage } from '@/common/services/api/api-error';

const props = defineProps<{ uid: string }>();
const open = defineModel<boolean>({ required: true });
const emit = defineEmits<{ saved: [] }>();

const password = ref('');
const errorText = ref('');
const saving = ref(false);

watch(open, (isOpen) => {
  if (isOpen) {
    password.value = '';
    errorText.value = '';
  }
});

async function save(): Promise<void> {
  saving.value = true;
  errorText.value = '';
  try {
    await apiService.users.setPassword(props.uid, password.value);
    emit('saved');
    open.value = false;
  } catch (caught) {
    errorText.value = errorMessage(caught);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <v-dialog v-model="open" max-width="420">
    <v-card title="Новый пароль" subtitle="Сервер сохранит его хэшем через операцию Password Modify">
      <v-card-text>
        <v-text-field v-model="password" label="Пароль" type="password" hint="Не короче 8 символов" autofocus />
        <v-alert v-if="errorText" type="error" variant="tonal">{{ errorText }}</v-alert>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn @click="open = false">Отмена</v-btn>
        <v-btn color="primary" :loading="saving" @click="save">Сохранить</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
