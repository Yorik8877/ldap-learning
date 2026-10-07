import type { AxiosInstance } from 'axios';
import type { DirectoryEntryDto, DirectoryNodeDto } from '../dtos';
import type { DirectoryEntry, DirectoryNode } from '../models';

export class DirectoryRepository {
  private readonly http: AxiosInstance;

  constructor(http: AxiosInstance) {
    this.http = http;
  }

  /** dn = null — потомки корня каталога (base DN). */
  async children(dn: string | null): Promise<DirectoryNode[]> {
    const { data } = await this.http.get<DirectoryNodeDto[]>('/directory/children', { params: dnParams(dn) });
    return data.map(toDirectoryNode);
  }

  async entry(dn: string | null): Promise<DirectoryEntry> {
    const { data } = await this.http.get<DirectoryEntryDto>('/directory/entry', { params: dnParams(dn) });
    return { dn: data.dn, attributes: data.attributes, operationalAttributes: data.operationalAttributes };
  }
}

function dnParams(dn: string | null): Record<string, string> {
  return dn === null ? {} : { dn };
}

function toDirectoryNode(dto: DirectoryNodeDto): DirectoryNode {
  return { dn: dto.dn, rdn: dto.rdn, objectClasses: dto.objectClass, hasChildren: dto.hasChildren };
}
