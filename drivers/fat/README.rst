FAT File System Driver
======================

The `FAT file system`_ has been around for decades and is still in widespread use
to this day.


Supported Features
------------------

====== ==== ===== ====== ======
System Read Write Create Delete
------ ---- ----- ------ ------
FAT 12 ✅
FAT 16 ✅
FAT 32 ✅
vFAT
====== ==== ===== ====== ======

Though Microsoft published the `exFAT specification`_ for free, it's covered by
a patent, and creating a driver requires purchasing a license.

Further Reading
---------------

* `Microsoft EFI FAT32 File System Specification`_
* `Design of the FAT file system`_

.. _FAT file system: https://en.wikipedia.org/wiki/File_Allocation_Table
.. _Design of the FAT file system: https://en.wikipedia.org/wiki/Design_of_the_FAT_file_system
.. _exFAT specification: https://docs.microsoft.com/en-us/windows/win32/fileio/exfat-specification
.. _Microsoft EFI FAT32 File System Specification: https://download.microsoft.com/download/1/6/1/161ba512-40e2-4cc9-843a-923143f3456c/fatgen103.doc
