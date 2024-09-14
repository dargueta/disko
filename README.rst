Disko: A Disk Image Editor
==========================

|go-versions| |platforms|

.. |go-versions| image:: https://img.shields.io/badge/Go-1.20,%201.21,%201.22,%201.23-blue.svg
.. |platforms| image::  https://img.shields.io/badge/platform-Linux%20%7C%20MacOS%20%7C%20Windows-lightgrey

I've recently gotten into retro computing and found I need to create disk images.
I also wanted to learn Go, so I figured I could kill two birds with one stone
and create a tool for doing this, written in Go.

You'll notice that there are a lot of ancient disk formats here, and nothing more
modern like extfs. This is deliberate; I need this for my retro computing projects,
so a lot of this is of no interest to anyone except those in the retro community.

Why the name? Disk + Go -> DiskGo -> Disko. Obviously.

API
---

Disko's ``Driver`` type requires a file system implementation with a minimal
interface. For details on the functions that need to be implemented, see
``DriverImplementation`` in ``api.go``.

**Symbols**

* ✔: Supported
* ⚠: Partial implementation, see notes
* ✘: Doesn't make sense for this, so the function deliberately wasn't implemented.

**Optional Features**

Implementations of file systems may need to provide support for specific features.
For convenience in the tables below, they are numbered thus:

1. Directories
2. File modes (Unix-style RWX)
3. File ownership (Unix-style UID/GID)
4. Timestamps
5. Hard links
6. Symbolic links


Base Driver
~~~~~~~~~~~

The base driver implements the following functions out of the box:

========= ======= ====================
Function  Support Required FS Features
========= ======= ====================
Chdir     ✔       1
Chmod     ✔       2
Chown     ✔       3
Chtimes   ✔       4
Create    ✔
Flush
Getwd     ✔
Lchown    ✔       3, 6
Link      ✔       5
Lstat     ✔
Mkdir     ✔
MkdirAll  ✔
Open      ✔
OpenFile  ✔
ReadDir   ✔
ReadFile  ✔
Readlink  ✔
Remove    ✔
RemoveAll ✔
Rename
SameFile  ✔
Stat      ✔
Symlink           6
Truncate  ✔
Unmount
Walk
WriteFile ✔
========= ======= ====================


Files
~~~~~

File handles support the following methods from ``os.File``:

================ ======= ====================
Function         Support Required FS Features
================ ======= ====================
Chdir            ✔       1
Chmod            ✔       2
Chown            ✔       3
Close            ✔
Fd               ✘
Name             ✔
Read             ✔
ReadAt           ✔
ReadDir          ✔       1
Readdir          ✔       1
Readdirnames     ✔       1
ReadFrom         ✔
Seek             ✔
SetDeadline      ✘
SetReadDeadline  ✘
SetWriteDeadline ✘
Stat             ✔
Sync             ✔
SyscallConn      ✘
Truncate         ✔
Write            ✔
WriteAt          ✔
WriteString      ✔
WriteTo          ✔
================ ======= ====================

File Systems
------------

The following table shows the file systems that drivers exist (or are planned)
for, as well as the status of the capabilities.

=============== ========== ================ ==== ==================== ================ ============
File System     Introduced Format New Image Read Write Existing Files Create New Files Delete Files
=============== ========== ================ ==== ==================== ================ ============
Unix v1 [#]_    1971       ✔
Unix v2         1972
Unix v5         1973
CP/M 1.4        1974
Unix v6         1975
FAT 8           1977       ✔
CP/M 2.2        1979
Unix v7         1979
FAT 12          1980
CP/M 3.1        1983
FAT 16          1984
CP/M 4.1 [#]_   1985
MINIX 3 [#]_    1987
Unix v10        1989
FAT 32          1996
XV6 (maybe)     2006
=============== ========== ================ ==== ==================== ================ ============

*Legend:*

* ✔: Full support
* ``B``: Beta, largely stable, may contain bugs
* ``A``: Alpha, use at your peril


CLI Features
------------

========================= ======
Feature                   Status
========================= ======
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

Development & Usage
-------------------

I make the following guarantees:

* Versioning strictly follows `the guidelines <https://go.dev/doc/modules/version-numbers>`_
  in Go's documentation.
* This is tested on:

  * At least the latest three minor versions of Go, e.g. if 1.19.x is the most recent
    release, I guarantee I'll test this on 1.17, 1.18, and 1.19; 1.16 and earlier are
    best-effort.
  * The latest versions of Ubuntu, Windows, and MacOS that are supported by
    GitHub.

Further Reading
---------------

* `UNIX v1 File System`_
*  `Full UNIX v1 Manual`_, relevant parts pages 171-174.
*  `Full UNIX v2 Manual`_, relevant parts pages 221-224.
*  `Full UNIX v5 Manual`_, relevant parts pages 237-238.
* `UNIX v6 File System`_
* `UNIX v10 File System`_
* `FAT 8`_, documenting FAT 8 on pages 172, 176, and 178.
* `FAT 12/16/32 on Wikipedia`_
* `CP/M file systems`_, including extensions.
* `MINIX 3 <https://flylib.com/books/en/3.275.1.54/1/>`_, shorter explanation `here <http://ohm.hgesser.de/sp-ss2012/Intro-MinixFS.pdf>`_.

.. _UNIX v1 File System: http://man.cat-v.org/unix-1st/5/file
.. _Full UNIX v1 Manual: http://www.bitsavers.org/pdf/bellLabs/unix/UNIX_ProgrammersManual_Nov71.pdf
.. _Full UNIX v2 Manual: https://web.archive.org/web/20161006034736/http://sunsite.icm.edu.pl/pub/unix/UnixArchive/PDP-11/Distributions/research/1972_stuff/unix_2nd_edition_manual.pdf
.. _Full UNIX v5 Manual: https://www.tuhs.org/Archive/Distributions/Research/Dennis_v5/v5man.pdf
.. _UNIX v6 File System: http://man.cat-v.org/unix-6th/5/fs
.. _UNIX v10 File System: http://man.cat-v.org/unix_10th/5/filsys
.. _FAT 12/16/32 on Wikipedia: https://en.wikipedia.org/wiki/File_Allocation_Table
.. _FAT 8: http://bitsavers.trailing-edge.com/pdf/xerox/820-II/BASIC-80_5.0.pdf
.. _CP/M file systems: https://www.seasip.info/Cpm/formats.html

License
-------

This is released under the terms of the AGPL v3 license (or later). Please see
the LICENSE file in this repository for the legal text.

Acknowledgments
---------------

This project uses open-source software built by other people, who have my
gratitude for building things so that I don't have to. [#]_

* `Bol Christophe <https://github.com/boljen>`_
* `gocarina <https://github.com/gocarina>`_
* `HashiCorp <https://github.com/hashicorp>`_
* `Tim Scheuermann <https://github.com/noxer>`_
* `urfave <https://github.com/urfave>`_

Footnotes
---------

.. [#] Timestamps are stored according to the 1973 revision that uses the canonical
       Unix epoch. The first version of the specification can't represent
       timestamps past 1973-04-08 12:06:28.250.
.. [#] Also known as "DOS Plus".
.. [#] Note this is version 3 of the file system, not MINIX version 3.
.. [#] This should not be taken to imply that any of the people or organizations
       listed here endorse or are associated with this project. It's just a thank you.
