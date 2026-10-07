<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { apiService } from '@/common/services/api/api.service';
import type { Group, User } from '@/common/services/api/models';
import { useNotifierStore } from '@/stores/notifier.store';
import { candidateMembers } from '../helpers/members';

const props = defineProps<{ name: string }>();

const notifier = useNotifierStore();
const router = useRouter();
const group = ref<Group | null>(null);
const users = ref<User[]>([]);
const newMember = ref<string | null>(null);
const confirmingDelete = ref(false);
const deleting = ref(false);

const candidates = computed(() => (group.value ? candidateMembers(users.value, group.value) : []));

async function load(): Promise<void> {
  try {
    [group.value, users.value] = await Promise.all([apiService.groups.get(props.name), apiService.users.list()]);
  } catch (caught) {
    notifier.error(caught);
  }
}

async function runAndReload(action: () => Promise<void>, successMessage: string): Promise<void> {
  try {
    await action();
    notifier.success(successMessage);
    await load();
  } catch (caught) {
    notifier.error(caught);
  }
}

function addMember(): void {
  const uid = newMember.value;
  if (!uid) {
    return;
  }
  newMember.value = null;
  void runAndReload(() => apiService.groups.addMember(props.name, uid), `${uid} добавлен в группу`);
}

function removeMember(uid: string): void {
  void runAndReload(() => apiService.groups.removeMember(props.name, uid), `${uid} убран из группы`);
}

async function remove(): Promise<void> {
  deleting.value = true;
  try {
    await apiService.groups.remove(props.name);
    notifier.success(`Группа ${props.name} удалена`);
    await router.push({ name: 'groups' });
  } catch (caught) {
    notifier.error(caught);
  } finally {
    deleting.value = false;
    confirmingDelete.value = false;
  }
}

watch(() => props.name, load, { immediate: true });
</script>

<template>
  <v-container>
    <v-card v-if="group" :title="group.name" :subtitle="group.description || 'без описания'">
      <v-card-text>
        <v-table density="compact">
          <thead>
            <tr>
              <th>uniqueMember (DN)</th>
              <th>uid</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr v-for="member in group.members" :key="member.dn">
              <td><code>{{ member.dn }}</code></td>
              <td>{{ member.uid ?? '— вне ou=people' }}</td>
              <td class="text-right">
                <v-btn
                  v-if="member.uid"
                  icon="mdi-account-remove"
                  size="small"
                  variant="text"
                  title="Убрать из группы"
                  @click="removeMember(member.uid)"
                />
              </td>
            </tr>
          </tbody>
        </v-table>
        <div class="d-flex ga-2 mt-4">
          <v-autocomplete
            v-model="newMember"
            :items="candidates"
            item-title="uid"
            item-value="uid"
            label="Добавить участника"
            density="compact"
            hide-details
          />
          <v-btn color="primary" :disabled="!newMember" @click="addMember">Добавить</v-btn>
        </div>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn color="error" prepend-icon="mdi-delete" @click="confirmingDelete = true">Удалить группу</v-btn>
      </v-card-actions>
    </v-card>
    <ConfirmDialog
      v-model="confirmingDelete"
      title="Удалить группу?"
      :text="`Запись cn=${name} будет удалена. У участников исчезнет memberOf этой группы.`"
      :loading="deleting"
      @confirm="remove"
    />
  </v-container>
</template>
