export interface CurrentUser {
  uid: string;
  commonName: string;
}

export interface User {
  uid: string;
  commonName: string;
  surname: string;
  emails: string[];
}

export interface UserDetails extends User {
  groups: string[];
}

export interface UserChanges {
  commonName: string;
  surname: string;
  emails: string[];
}

export interface UserDraft extends UserChanges {
  uid: string;
  password: string;
}

export interface GroupSummary {
  name: string;
  description: string;
  memberCount: number;
}

export interface GroupMember {
  dn: string;
  /** null — участник вне ou=people, у него нет uid. */
  uid: string | null;
}

export interface Group {
  name: string;
  description: string;
  members: GroupMember[];
}

export interface GroupDraft {
  name: string;
  description: string;
  memberUids: string[];
}

export interface DirectoryNode {
  dn: string;
  rdn: string;
  objectClasses: string[];
  hasChildren: boolean;
}

export interface DirectoryEntry {
  dn: string;
  attributes: Record<string, string[]>;
  operationalAttributes: Record<string, string[]>;
}
