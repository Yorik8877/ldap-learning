import { ref } from 'vue';
import { apiService } from '@/common/services/api/api.service';
import type { DirectoryEntry, DirectoryNode } from '@/common/services/api/models';
import type { DirectoryRepository } from '@/common/services/api/repositories/directory.repository';

export interface TreeItem {
  id: string;
  title: string;
  children?: TreeItem[];
}

type TreeSource = Pick<DirectoryRepository, 'children' | 'entry'>;

// Пустой массив children — сигнал VTreeview: у узла есть потомки, загрузить их при раскрытии.
// Без поля children узел считается листом и стрелки раскрытия у него нет.
export function toTreeItem(node: DirectoryNode): TreeItem {
  const item: TreeItem = { id: node.dn, title: node.rdn };
  if (node.hasChildren) {
    item.children = [];
  }
  return item;
}

export function useDirectoryTree(repository: TreeSource = apiService.directory) {
  const items = ref<TreeItem[]>([]);
  const selected = ref<DirectoryEntry | null>(null);

  async function loadRoot(): Promise<void> {
    const root = await repository.entry(null);
    items.value = [{ id: root.dn, title: root.dn, children: [] }];
  }

  async function loadChildren(item: TreeItem): Promise<void> {
    const nodes = await repository.children(item.id);
    item.children?.push(...nodes.map(toTreeItem));
  }

  async function select(dn: string): Promise<void> {
    selected.value = await repository.entry(dn);
  }

  return { items, selected, loadRoot, loadChildren, select };
}
