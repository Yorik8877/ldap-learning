<script setup lang="ts">
defineProps<{ title: string; text: string; confirmLabel?: string; loading?: boolean }>();
const open = defineModel<boolean>({ required: true });
const emit = defineEmits<{ confirm: [] }>();
</script>

<template>
  <v-dialog v-model="open" max-width="480">
    <v-card :title="title" :text="text">
      <v-card-actions>
        <v-spacer />
        <v-btn @click="open = false">Отмена</v-btn>
        <v-btn color="error" :loading="loading" @click="emit('confirm')">{{ confirmLabel ?? 'Удалить' }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
