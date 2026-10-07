<script setup lang="ts">
import type { DirectoryEntry } from '@/common/services/api/models';
import AttributeTable from './AttributeTable.vue';

defineProps<{ entry: DirectoryEntry }>();
</script>

<template>
  <div>
    <v-alert type="info" variant="tonal" class="mb-4">
      <div class="text-caption">DN</div>
      <code>{{ entry.dn }}</code>
    </v-alert>
    <AttributeTable title="Атрибуты записи" :attributes="entry.attributes" />
    <AttributeTable title="Служебные атрибуты (ведёт сервер)" :attributes="entry.operationalAttributes" />
  </div>
</template>
