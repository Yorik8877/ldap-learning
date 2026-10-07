<script setup lang="ts">
import { storeToRefs } from 'pinia';
import { useNotifierStore } from '@/stores/notifier.store';

const { visible, text, kind } = storeToRefs(useNotifierStore());
</script>

<template>
  <v-snackbar v-model="visible" :color="kind" :timeout="6000" location="top">{{ text }}</v-snackbar>
</template>
