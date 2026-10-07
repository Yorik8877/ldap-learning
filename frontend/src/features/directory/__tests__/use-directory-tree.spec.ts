import { describe, expect, it, vi } from 'vitest';
import type { DirectoryEntry, DirectoryNode } from '@/common/services/api/models';
import { toTreeItem, useDirectoryTree } from '../composables/use-directory-tree';

const people: DirectoryNode = { dn: 'ou=people,dc=example,dc=com', rdn: 'ou=people', objectClasses: [], hasChildren: true };
const alice: DirectoryNode = { dn: 'uid=alice,ou=people,dc=example,dc=com', rdn: 'uid=alice', objectClasses: [], hasChildren: false };
const rootEntry: DirectoryEntry = { dn: 'dc=example,dc=com', attributes: {}, operationalAttributes: {} };

function fakeRepository() {
  return {
    children: vi.fn().mockResolvedValue([people]),
    entry: vi.fn().mockResolvedValue(rootEntry),
  };
}

describe('toTreeItem', () => {
  it('marks loadable nodes with an empty children array and leaves leaves without it', () => {
    expect(toTreeItem(people).children).toEqual([]);
    expect('children' in toTreeItem(alice)).toBe(false);
  });
});

describe('useDirectoryTree', () => {
  it('starts from the base entry and loads children on demand', async () => {
    const repository = fakeRepository();
    const tree = useDirectoryTree(repository);

    await tree.loadRoot();
    const [root] = tree.items.value;
    expect(root?.id).toBe('dc=example,dc=com');

    if (!root) throw new Error('root item is missing');
    await tree.loadChildren(root);

    expect(repository.children).toHaveBeenCalledWith('dc=example,dc=com');
    expect(root.children?.map((child) => child.id)).toEqual(['ou=people,dc=example,dc=com']);
  });

  it('selects an entry by dn', async () => {
    const repository = fakeRepository();
    const tree = useDirectoryTree(repository);

    await tree.select('dc=example,dc=com');

    expect(repository.entry).toHaveBeenCalledWith('dc=example,dc=com');
    expect(tree.selected.value).toEqual(rootEntry);
  });
});
