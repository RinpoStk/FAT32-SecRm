# FAT32-SecRm
A Tool to remove file in FAT32 filesystem

软件安全作业，使用go语言编写。

## 描述

- 该工具会清空指定文件内容，删除其占用的FAT32表簇号，并把目录项标记为已删除(0xe5)
- 支持删除文件夹，工具会递归地删除文件夹下的子文件与所有文件

## 多平台

支持Windows与Linux平台，其他平台可以通过实现driver接口内的读取写入扇区适配。
