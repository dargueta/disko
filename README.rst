Disko: A Disk Image Editor
==========================

I've recently gotten into retro computing and found I need to create disk images.
I also wanted to learn Go, so I figured I could kill two birds with one stone
and create a tool for doing this, written in Go.

You'll notice that there are a lot of ancient disk formats here, and nothing more
modern like extfs. This is deliberate; I need this for my retro computing projects,
so a lot of this is of no interest to anyone except those in the retro community.

Why the name? Disk + Go -> DiskGo -> Disko. Obviously.

API
---

At time of writing (July 2022) I'm doing an overhaul of the API. Ignore most of
``api.go``; I've managed to implement most of the ``Driver`` interface while
needing a much simpler (albeit more abstract) interface from file system
implementations. This should greatly simplify adding new drivers.


File Systems
------------

The following table shows the file systems that drivers exist (or are planned)
for, as well as the status of the capabilities.

=============== ================ ==== ==================== ================ ============
File System     Format New Image Read Write Existing Files Create New Files Delete Files
--------------- ---------------- ---- -------------------- ---------------- ------------
CP/M 1.4
CP/M 2.2
CP/M 3.1
CP/M 4.1
FAT 8           B [#]_
FAT 12
FAT 16
FAT 32
MINIX
Unix V1FS [#]_  ✔
Unix V6FS
Unix V7FS
Unix V10FS
XV6
=============== ================ ==== ==================== ================ ============

*Legend:*

* ✔: Full support
* ``B``: Beta, largely stable, may contain bugs
* ``A``: Alpha, use at your peril


CLI Features
------------

========================= ======
Feature                   Status
------------------------- ------
Create blank image
List files
Insert individual files
Insert directory trees
Remove individual files
Remove using shell globs
Remove trees
Extract individual files
Extract directory trees
Extract using shell globs
Interactive editing
========================= ======


License
-------

Against my better judgement I'm open-sourcing this footgun for anyone to use,
albeit at their own peril. This is released under the terms of the Apache 2.0
License. Please see LICENSE.txt in this repository for the legal text.

Acknowledgments
---------------

This project uses the following third-party packages in accordance with their
licenses. A project's appearance in this list does not imply endorsement by or
affiliation with the author.

* `cli <github.com/urfave/cli>`_ by urfave
* `go-bitmap <https://github.com/boljen/go-bitmap>`_ by Bol Christophe

Footnotes
---------

.. [#] Works for the larger image size; smaller image size is buggy.
.. [#] Timestamps are stored using the 1973 revision that uses the canonical
       Unix epoch. The first specification can't represent timestamps past
       1973-04-08 12:06:28.250.
