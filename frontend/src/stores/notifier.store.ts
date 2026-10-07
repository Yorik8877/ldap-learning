import { defineStore } from 'pinia';
import { ref } from 'vue';
import { errorMessage } from '@/common/services/api/api-error';

export type NoticeKind = 'success' | 'error';

export const useNotifierStore = defineStore('notifier', () => {
  const visible = ref(false);
  const text = ref('');
  const kind = ref<NoticeKind>('success');

  function show(message: string, noticeKind: NoticeKind): void {
    text.value = message;
    kind.value = noticeKind;
    visible.value = true;
  }

  function success(message: string): void {
    show(message, 'success');
  }

  function error(caught: unknown): void {
    show(errorMessage(caught), 'error');
  }

  return { visible, text, kind, success, error };
});
