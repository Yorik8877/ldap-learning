import '@mdi/font/css/materialdesignicons.css';
import 'vuetify/styles';
import { createVuetify } from 'vuetify';
import { ru } from 'vuetify/locale';

export const vuetify = createVuetify({
  locale: { locale: 'ru', messages: { ru } },
  theme: { defaultTheme: 'light' },
});
