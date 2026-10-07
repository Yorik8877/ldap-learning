<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { apiService } from '@/common/services/api/api.service';
import { errorMessage } from '@/common/services/api/api-error';
import type { User } from '@/common/services/api/models';
import { formatEmails, parseEmails } from '../helpers/emails';

const props = defineProps<{ user: User | null }>();
const open = defineModel<boolean>({ required: true });
const emit = defineEmits<{ saved: [user: User] }>();

const uid = ref('');
const commonName = ref('');
const surname = ref('');
const emails = ref('');
const password = ref('');
const errorText = ref('');
const saving = ref(false);
const isEditing = computed(() => props.user !== null);

watch(open, (isOpen) => {
  if (isOpen) {
    reset();
  }
});

function reset(): void {
  uid.value = props.user?.uid ?? '';
  commonName.value = props.user?.commonName ?? '';
  surname.value = props.user?.surname ?? '';
  emails.value = formatEmails(props.user?.emails ?? []);
  password.value = '';
  errorText.value = '';
}

function persist(): Promise<User> {
  const changes = { commonName: commonName.value, surname: surname.value, emails: parseEmails(emails.value) };
  if (props.user) {
    return apiService.users.update(props.user.uid, changes);
  }
  return apiService.users.create({ uid: uid.value.trim(), password: password.value, ...changes });
}

async function save(): Promise<void> {
  saving.value = true;
  errorText.value = '';
  try {
    emit('saved', await persist());
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
    <v-card :title="isEditing ? 'Редактирование пользователя' : 'Новый пользователь'">
      <v-card-text>
        <v-text-field v-model="uid" label="uid" :disabled="isEditing" hint="Строчные латинские буквы, цифры, «.», «_», «-»" />
        <v-text-field v-model="commonName" label="cn — полное имя" />
        <v-text-field v-model="surname" label="sn — фамилия" />
        <v-text-field v-model="emails" label="mail — адреса через запятую" />
        <v-text-field v-if="!isEditing" v-model="password" label="Пароль" type="password" hint="Не короче 8 символов" />
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
