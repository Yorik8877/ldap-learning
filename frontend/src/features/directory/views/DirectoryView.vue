<script setup lang="ts">
import { onMounted } from 'vue';
import { useNotifierStore } from '@/stores/notifier.store';
import EntryDetails from '../components/EntryDetails.vue';
import { type TreeItem, useDirectoryTree } from '../composables/use-directory-tree';

const notifier = useNotifierStore();
const tree = useDirectoryTree();
const { items, selected } = tree;

async function guarded(action: () => Promise<void>): Promise<void> {
  try {
    await action();
  } catch (caught) {
    notifier.error(caught);
  }
}

// VTreeview передаёт исходный объект элемента, то есть наш TreeItem.
function loadChildren(item: unknown): Promise<void> {
  return guarded(() => tree.loadChildren(item as TreeItem));
}

function onActivated(activated: unknown): void {
  const [dn] = Array.isArray(activated) ? activated : [];
  if (typeof dn === 'string') {
    void guarded(() => tree.select(dn));
  }
}

onMounted(() => guarded(tree.loadRoot));
</script>

<template>
  <v-container fluid>
    <v-row>
      <v-col cols="12" md="5">
        <v-card title="Дерево каталога">
          <v-treeview
            :items="items"
            item-value="id"
            item-title="title"
            :load-children="loadChildren"
            activatable
            density="compact"
            @update:activated="onActivated"
          />
        </v-card>
      </v-col>
      <v-col cols="12" md="7">
        <EntryDetails v-if="selected" :entry="selected" />
        <v-alert v-else type="info" variant="tonal">Выберите запись в дереве, чтобы увидеть её атрибуты.</v-alert>
      </v-col>
    </v-row>
  </v-container>
</template>
