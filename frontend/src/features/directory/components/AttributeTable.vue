<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{ title: string; attributes: Record<string, string[]> }>();

const rows = computed(() =>
  Object.entries(props.attributes)
    .map(([name, values]) => ({ name, values }))
    .sort((left, right) => left.name.localeCompare(right.name)),
);
</script>

<template>
  <v-card :title="title" variant="outlined" class="mb-4">
    <v-table density="compact">
      <tbody>
        <tr v-for="row in rows" :key="row.name">
          <td class="font-weight-medium">{{ row.name }}</td>
          <td>
            <div v-for="(value, index) in row.values" :key="index">{{ value }}</div>
          </td>
        </tr>
      </tbody>
    </v-table>
  </v-card>
</template>
