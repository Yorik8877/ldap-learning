// Формы ответов бэкенда как есть: имена полей совпадают с атрибутами LDAP.

export interface CurrentUserDto {
  uid: string;
  cn: string;
}

export interface UserDto {
  uid: string;
  cn: string;
  sn: string;
  mail: string[];
}

export interface UserDetailsDto extends UserDto {
  groups: string[];
}

export interface GroupSummaryDto {
  cn: string;
  description: string;
  memberCount: number;
}

export interface GroupMemberDto {
  dn: string;
  uid: string;
}

export interface GroupDto {
  cn: string;
  description: string;
  members: GroupMemberDto[];
}

export interface DirectoryNodeDto {
  dn: string;
  rdn: string;
  objectClass: string[];
  hasChildren: boolean;
}

export interface DirectoryEntryDto {
  dn: string;
  attributes: Record<string, string[]>;
  operationalAttributes: Record<string, string[]>;
}
