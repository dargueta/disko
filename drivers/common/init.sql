PRAGMA foreign_keys = ON;

CREATE TABLE inodes (
    id INTEGER PRIMARY KEY,
    size BIGINT DEFAULT 0,
    nlinks INTEGER DEFAULT 1,
    t_created TEXT,
    t_accessed TEXT,
    t_modified TEXT,
    t_changed TEXT,
    t_deleted TEXT,
    uid INTEGER,
    gid INTEGER,
    mode_flags INTEGER,

    -- fs_specific_flags is a bitfield of flags specific to the file system, and
    -- are ignored by generic driver abstraction layers. Modifications to an
    -- inode by a generic driver must leave this unchanged.
    fs_specific_flags INTEGER
);


CREATE TABLE dirents (
    id INTEGER PRIMARY KEY,
    "name" TEXT NOT NULL,
    -- Despite being a foreign key, we're not creating an index on the inode ID
    -- since we're extremely unlikely to search across directory entries for an
    -- inode, as opposed to the other way around (finding the inode of a
    -- directory entry).
    --
    -- Because we have a unique index on the parent ID and name (in that order),
    -- we don't need to create a separate index for just the parent ID, as the
    -- engine can use that unique constraint for a partial index search.
    inode_id INTEGER NOT NULL REFERENCES inodes(id) ON DELETE CASCADE,
    -- Only directory entries in the root directory are allowed to have this null,
    -- and only on certain file systems like FAT8/12/16. It's up to individual
    -- drivers to ensure the value here is correct.
    parent_dirent_id INTEGER REFERENCES dirents(id) ON DELETE CASCADE,
    UNIQUE(parent_dirent_id, "name")
);


CREATE TABLE allocated_blocks (
    id INTEGER PRIMARY KEY,
    inode_id INTEGER REFERENCES inodes(id) ON DELETE CASCADE,
    block_id INTEGER NOT NULL,
    sequence_number INTEGER NOT NULL,
    indirection_level INTEGER DEFAULT 0,
    -- Same note as earlier -- no need to create a separate index on the inode
    -- ID, since we can use the unique index for this.
    UNIQUE(inode_id, sequence_number)
);


-- Decrement `nlinks`` of an inode when a directory entry that references it is
-- deleted.
CREATE TRIGGER trg_decrement_nlinks
AFTER DELETE ON dirents
BEGIN
    UPDATE inodes
    SET
        -- Decrement the reference count
        nlinks = nlinks - 1,
        -- Automatically set the deleted timestamp if the reference count is
        -- about to drop below 1. Otherwise, leave the column unmodified.
        t_deleted = CASE
            WHEN nlinks <= 1 THEN datetime('now')
            ELSE t_deleted
        END
    WHERE id = OLD.inode_id;
END;


-- Deallocate all the blocks of an inode when the number of references to it
-- reaches zero.
CREATE TRIGGER trg_deallocate_blocks
AFTER UPDATE OF nlinks ON inodes WHEN nlinks <= 0
BEGIN
    DELETE FROM allocated_blocks WHERE inode_id = OLD.id;
END;
