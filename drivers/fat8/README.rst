FAT8 File System Driver
=======================

This is a driver for the FAT8 file system, a predecessor of the FAT versions used
to this day. I've only been able to find documentation for its structure on two
disk geometries.

================= ====== ============= ============ ==================
Medium            Tracks Sectors/Track Bytes/Sector Formatted Capacity
----------------- ------ ------------- ------------ ------------------
8-inch floppy     73     26            128          234 KiB
5 1/4-inch floppy 40     16            128          78 KiB
================= ====== ============= ============ ==================

Source: `Xerox BASIC-80 Reference Manual <http://bitsavers.trailing-edge.com/pdf/xerox/820-II/BASIC-80_5.0.pdf>`_,
pages 172, 176, and 178.
