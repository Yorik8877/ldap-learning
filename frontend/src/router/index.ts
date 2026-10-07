import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router';
import { LoginView } from '@/features/auth';
import { DirectoryView } from '@/features/directory';
import DefaultLayout from '@/layouts/DefaultLayout.vue';
import HomeView from '@/views/HomeView.vue';
import { authGuard } from './guards';
import './types';

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: LoginView, meta: { publicAccess: true } },
  {
    path: '/',
    component: DefaultLayout,
    children: [
      { path: '', name: 'home', component: HomeView },
      { path: 'directory', name: 'directory', component: DirectoryView },
    ],
  },
];

export const router = createRouter({ history: createWebHistory(), routes });

router.beforeEach(authGuard);
