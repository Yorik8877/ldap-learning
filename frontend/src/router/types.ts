import 'vue-router';

declare module 'vue-router' {
  interface RouteMeta {
    publicAccess?: boolean;
  }
}

export {};
