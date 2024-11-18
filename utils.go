package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	FATBuffer    *FAT32Buffer
	DEntryBuffer *DirEntryBuffer
)

const BufferSize = 32

// fileNameEqual 短文件名目录项比较
func fileNameEqual(dEntryName []byte, fileName string) (bool, error) {
	fileExt := filepath.Ext(fileName)
	fileBase := strings.TrimSuffix(fileName, fileExt)
	dEntryFileBase := strings.Replace(string(dEntryName[:8]), " ", "", -1)
	dEntryFileExt := strings.Replace(string(dEntryName[8:]), " ", "", -1)
	// 删除拓展名的点
	if dEntryFileBase == strings.ToUpper(fileBase) &&
		dEntryFileExt == strings.ToUpper(fileExt[1:]) {
		return true, nil
	} else {
		return false, nil
	}
}

// listFiles 返回一个目录下所有的子目录与文件，如有子目录会同时进入列出
func listFiles(root string) ([]string, error) {
	var files []string
	// 使用 Walk 函数递归遍历目录和文件
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 只添加文件和目录的绝对路径
		if path != root { // 排除根目录
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			files = append(files, absPath)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 将根目录添加到最后
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	files = append(files, rootAbs)

	return files, nil
}

// getDirEntry 依据路径获取最后一个目录项与目录项对应的偏移
func getDirEntry(driver *DefalutDriver, filePath string) (*FAT32DirEntry, []uint16, error) {
	filePathArr := strings.Split(filePath, Segment)
	var dEntryOffset []uint16
	dEntryLL, err := getFATLink(driver, 2)
	if err != nil {
		return nil, nil, err
	}
	dentry := &FAT32DirEntry{ClusterLow: 2}
	for i, name := range filePathArr {
		if i == len(filePathArr)-1 {
			// 最后一个
			var tdentry *FAT32DirEntry
			tdentry, dEntryOffset, err = findDirEntry(driver, dEntryLL, dentry, name)
			dEntryOffset = append([]uint16{dentry.ClusterHigh, dentry.ClusterLow}, dEntryOffset...)
			dentry = tdentry
		} else {
			dentry, _, err = findDirEntry(driver, dEntryLL, dentry, name)
		}
		if err != nil {
			return nil, nil, err
		}
	}
	return dentry, dEntryOffset, nil
}

// findDirEntry 依据文件名搜索目录项
func findDirEntry(driver *DefalutDriver, dEntryLL []uint32, currentDentry *FAT32DirEntry, targetFile string) (*FAT32DirEntry, []uint16, error) {
	const dEntryChunkSize = 32

	// for i, n := range dEntryLL {
	//	if i%BufferSize == 0 {
	//		DEntryBuffer.Number =
	//	}
	// }

	// var newDEntry FAT32DirEntry
	// var dEntryName []uint16
	// var dEntryOffset []uint16

	// // 依据簇号链表获取完整目录项
	// offsetCluster := uint32(currentDentry.ClusterHigh)<<16 + uint32(currentDentry.ClusterLow) - 2
	// buffer, err := driver.ReadSector(
	//	uint64(driver.Offset.buffer)+uint64(offsetCluster)*uint64(driver.BPRSector.SectorsPerCluster),
	//	uint16(driver.BPRSector.SectorsPerCluster),
	// )
	// if err != nil {
	//	return nil, nil, err
	// }
	// // 匹配每个目录项
	// for i := 0; i < len(buffer); i += dEntryChunkSize {
	//	if i+dEntryChunkSize > len(buffer) {
	//		break
	//	}
	//	chunk := buffer[i : i+dEntryChunkSize]
	//
	//	if chunk[0] == 0 || chunk[0] == 229 {
	//		// 跳过未使用的目录项与已删除项
	//		continue
	//	}
	//
	//	switch chunk[11] {
	//	case 16, 32:
	//		err = binary.Read(bytes.NewReader(chunk), binary.LittleEndian, &newDEntry)
	//		if err != nil {
	//			return nil, nil, err
	//		}
	//		// 长文件名
	//		if dEntryName != nil {
	//			// 截取到为0的元素
	//			for j := 0; j < len(dEntryName); j++ {
	//				if dEntryName[j] == 0 {
	//					dEntryName = dEntryName[:j]
	//					break
	//				}
	//			}
	//			// fmt.Println(string(utf16.Decode(dEntryName)))
	//			if string(utf16.Decode(dEntryName)) == targetFile {
	//				dEntryOffset = append(dEntryOffset, uint16(i))
	//				return &newDEntry, dEntryOffset, nil
	//			}
	//			dEntryName = nil
	//			dEntryOffset = []uint16{}
	//			// 长文件名不一致，证明不是该目录项，无需比较短文件名目录项
	//			continue
	//		}
	//		// fmt.Println(string(newDEntry.FileName[:11]))
	//
	//		isEqual, err := FileNameEqual(newDEntry.FileName[:11], targetFile)
	//		if err != nil {
	//			return nil, nil, err
	//		}
	//		if isEqual {
	//			dEntryOffset = []uint16{uint16(i)}
	//			return &newDEntry, dEntryOffset, nil
	//		}
	//	case 15:
	//		// 解析长文件名项
	//		var newLDEntry FAT32LongDirEntry
	//		err = binary.Read(bytes.NewReader(chunk), binary.LittleEndian, &newLDEntry)
	//		if err != nil {
	//			return nil, nil, err
	//		}
	//		if newLDEntry.SequenceNumber&0x40 == 0x40 {
	//			// 依据长度创建文件名切片
	//			dEntryName = make([]uint16, (newLDEntry.SequenceNumber&0x1f)*13)
	//		}
	//		// 将长文件名项写入文件名切片
	//		tmpName := append(newLDEntry.Name1[:5], newLDEntry.Name2[:6]...)
	//		tmpName = append(tmpName, newLDEntry.Name3[:2]...)
	//		copy(dEntryName[13*((newLDEntry.SequenceNumber&0x1f)-1):], tmpName)
	//		// 将长文件名偏移写入
	//		dEntryOffset = append(dEntryOffset, uint16(i))
	//	}
	//
	// }
	return nil, nil, errors.New("not found")
}

// readFATEntry 读取某号fat表项指向的fat表项
func readFATEntry(driver *DefalutDriver, FATEntry uint32) (uint32, error) {
	fatOffset := FATEntry * 4 / uint32(driver.BPRSector.BytesPerSector)
	fatBufferOffset := fatOffset % 32
	fatBufferBase := fatOffset - fatBufferOffset
	// 更新fat32表缓冲区
	if fatBufferBase != FATBuffer.Number {
		err := UpdateFAT(driver, fatBufferBase)
		if err != nil {
			return 0, err
		}
	}
	entryOffset := FATEntry - fatBufferBase*uint32(driver.BPRSector.BytesPerSector)
	return FATBuffer.Link[entryOffset], nil
}

// getFATLink 依据一条fat表项获取整个fat link
func getFATLink(driver *DefalutDriver, FATEntry uint32) ([]uint32, error) {

	// for i:=FATBuffer;i!=0x0fffffff;i=

	i, err := readFATEntry(driver, FATEntry)
	if err != nil {
		return nil, err
	}
	res := []uint32{i}
	for i != 0x0fffffff {
		i, err = readFATEntry(driver, i)
		if err != nil {
			return nil, err
		}
		res = append(res, i)
	}
	fmt.Println(res)
	return res, nil
}

// getDriveFactory driver工厂函数，返回driver实例
func getDriveFactory(absFileName string) (*DefalutDriver, error) {
	var driver DefalutDriver
	err := driver.DInit(absFileName)
	if err != nil {
		return nil, err
	}
	return &driver, nil
}

// UpdateFAT 更新fat32表缓冲区
func UpdateFAT(driver *DefalutDriver, fatOffset uint32) error {
	buffer, err := driver.ReadSector(uint64(driver.Offset.DEntry)+uint64(fatOffset), BufferSize)
	if err != nil {
		return err
	}
	fat32l := make([]uint32, len(buffer)/4)
	// 逐个解析每 4 字节为一个 uint32
	for i := 0; i < len(fat32l); i++ {
		fat32l[i] = binary.LittleEndian.Uint32(buffer[i*4 : (i+1)*4])
	}
	FATBuffer.Number = fatOffset
	FATBuffer.Link = fat32l
	return nil
}

// DeleteFile 删除文件或文件夹
func DeleteFile(absFileName string) error {
	FATBuffer = &FAT32Buffer{}
	DEntryBuffer = &DirEntryBuffer{}
	driver, err := getDriveFactory(absFileName)
	if err != nil {
		return err
	}

	dentry, dEntryOffset, err := getDirEntry(driver, absFileName)
	if err != nil {
		return err
	}
	if dentry.FileSize == 0 {
		// 空文件直接删除即可
		return nil
	}
	fmt.Println(dEntryOffset)
	// fatOffset := (uint32(dentry.ClusterHigh) << 16) + uint32(dentry.ClusterLow)
	// fmt.Println(fatOffset)
	// fatOffset = fatOffset * 4 / uint32(BPRSector.BytesPerSector)
	// fmt.Println(fatOffset)
	// buffer, err := winReadSector(uint64(fatOffset)+uint64(Offset.DEntry), 16)
	// if err != nil {
	//	return err
	// }
	// fat32l := make([]uint32, len(buffer)/4)
	// // 逐个解析每 4 字节为一个 uint32
	// for i := 0; i < len(fat32l); i++ {
	//	fat32l[i] = binary.LittleEndian.Uint32(buffer[i*4 : (i+1)*4])
	// }
	// fat32t := FAT32Table{fatOffset, fat32l}
	// fmt.Println(string(dentry.FileName[:11]))
	// fmt.Println(dEntryOffset)
	// fmt.Println(fat32t)
	return driver.DDestroy()
}
