Disko -- A Disk Image Manager
=============================

This is purely for the purpose of my learning Go and I don't want to have just one copy
on my hard drive. This should not be taken seriously at all. Don't use it.

File Systems
------------

The following table shows the file systems that drivers exist (or are planned)
for, as well as the status of the capabilities. You may notice that there are a
lot of old formats here; this is because I've recently gotten into retro computing
and want to be able to play around with old systems.

=========== ================ ==== ==================== ================ ============
File System Format New Image Read Write Existing Files Create New Files Delete Files
----------- ---------------- ---- -------------------- ---------------- ------------
CP/M 1.4
CP/M 2.2
CP/M 3.1
CP/M 4.1
FAT 8       ✅ [#]_           ✅
FAT 12
FAT 16
FAT 32
MINIX
Unix V1FS
Unix V6FS
Unix V7FS
Unix V10FS
=========== ================ ==== ==================== ================ ============


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

Against my better judgement I'm making this open-source for anyone to use at their own
peril. This is released under the terms of the BSD 3-Clause License. Please see
LICENSE.txt in this repository for the legal text.

Acknowledgments
---------------

This project uses the following third-party packages in accordance with their
licenses. A project's appearance in this list does not imply endorsement by or
affiliation with the author.

* `go-bitmap <https://github.com/boljen/go-bitmap>`_ by Bol Christophe


Footnotes
---------

.. [#] Works for the larger image size; smaller image size is buggy.
