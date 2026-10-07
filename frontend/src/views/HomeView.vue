<template>
  <v-container>
    <v-card title="Учебная админ-панель OpenLDAP">
      <v-card-text>
        <p>Разделы в меню сверху:</p>
        <ul class="ml-6 mt-2">
          <li>«Дерево» — каталог как есть: записи, их objectClass и атрибуты, включая служебные.</li>
          <li>«Пользователи» — записи inetOrgPerson в ou=people.</li>
          <li>«Группы» — записи groupOfUniqueNames в ou=groups.</li>
        </ul>
      </v-card-text>
    </v-card>
  </v-container>
</template>
