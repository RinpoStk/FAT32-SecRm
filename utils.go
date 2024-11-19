package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
)

var FATBuffer *FAT32Buffer

const FAT32BufferSize = 32

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
func getDirEntry(driver *DefaultDriver, filePath string) (*FAT32DirEntry, []*DirEntryOffset, error) {
	filePathArr := strings.Split(filePath, Segment)
	dEntry := &FAT32DirEntry{ClusterLow: 2}
	var dEntryOffset []*DirEntryOffset

	for _, name := range filePathArr {
		dEntryLL, err := getFATLink(driver, (uint32(dEntry.ClusterHigh)<<16)+uint32(dEntry.ClusterLow))
		if err != nil {
			return nil, nil, err
		}
		dEntry, dEntryOffset, err = findDirEntry(driver, dEntryLL, name)
		if err != nil {
			return nil, nil, err
		}

	}
	return dEntry, dEntryOffset, nil
}

// findDirEntry 依据文件名搜索目录项
func findDirEntry(driver *DefaultDriver, dEntryLL []uint32, targetFile string) (*FAT32DirEntry, []*DirEntryOffset, error) {
	const dEntryChunkSize = 32

	for _, cluster := range dEntryLL {
		// 结束
		if cluster == 0x0fffffff {
			break
		}
		// 依据簇号链表获取完整目录项
		buffer, err := driver.ReadSector(
			uint64(driver.Offset.Data)+uint64(cluster-2)*uint64(driver.BPRSector.SectorsPerCluster),
			uint16(driver.BPRSector.SectorsPerCluster),
		)
		if err != nil {
			return nil, nil, err
		}

		var dEntry FAT32DirEntry
		var dEntryName []uint16
		var dEntryOffset []*DirEntryOffset

		// 匹配每个目录项
		for i := 0; i < len(buffer); i += dEntryChunkSize {
			if i+dEntryChunkSize > len(buffer) {
				break
			}
			chunk := buffer[i : i+dEntryChunkSize]

			if chunk[0] == 0 || chunk[0] == 229 {
				// 跳过未使用的目录项与已删除项
				continue
			}

			switch chunk[11] {
			case 16, 32: // 短文件名目录项
				err = binary.Read(bytes.NewReader(chunk), binary.LittleEndian, &dEntry)
				if err != nil {
					return nil, nil, err
				}
				// 长文件名
				if dEntryName != nil {
					// 截取到为0的元素
					for j := 0; j < len(dEntryName); j++ {
						if dEntryName[j] == 0 {
							dEntryName = dEntryName[:j]
							break
						}
					}
					if string(utf16.Decode(dEntryName)) == targetFile {
						dEntryOffset = append(dEntryOffset, &DirEntryOffset{
							cluster,
							uint16(i),
						})
						return &dEntry, dEntryOffset, nil
					}
					dEntryName = nil
					dEntryOffset = []*DirEntryOffset{}
					// 长文件名不一致，证明不是该目录项，无需比较短文件名目录项
					continue
				}

				isEqual, err := fileNameEqual(dEntry.FileName[:11], targetFile)
				if err != nil {
					return nil, nil, err
				}
				if isEqual {
					dEntryOffset = []*DirEntryOffset{
						{
							cluster,
							uint16(i),
						},
					}
					return &dEntry, dEntryOffset, nil
				}
			case 15: // 解析长文件名项
				var lDEntry FAT32LongDirEntry
				err = binary.Read(bytes.NewReader(chunk), binary.LittleEndian, &lDEntry)
				if err != nil {
					return nil, nil, err
				}
				// 长文件名结束
				if lDEntry.SequenceNumber&0x40 == 0x40 {
					// 依据长度创建文件名切片
					dEntryName = make([]uint16, (lDEntry.SequenceNumber&0x1f)*13)
				}
				// 将长文件名项写入文件名切片
				tmpName := append(lDEntry.Name1[:5], lDEntry.Name2[:6]...)
				tmpName = append(tmpName, lDEntry.Name3[:2]...)
				copy(dEntryName[13*((lDEntry.SequenceNumber&0x1f)-1):], tmpName)
				// 将长文件名偏移写入
				dEntryOffset = append(dEntryOffset, &DirEntryOffset{
					cluster,
					uint16(i),
				})
			}

		}
	}
	return nil, nil, errors.New("not found")
}

func doRemoveFile(driver *DefaultDriver, dEntry *FAT32DirEntry, dEntryOffsets []*DirEntryOffset) error {
	for i, offset := range dEntryOffsets {
		if i != len(dEntryOffsets)-1 {
			err := rmDEntry(driver, offset)
			if err != nil {
				return err
			}
		} else {
			if dEntry.ClusterHigh == 0 && dEntry.ClusterLow == 0 { // 空文件
				err := rmDEntry(driver, offset)
				if err != nil {
					return err
				}
			} else {
				fat32LL, err := getFATLink(driver, (uint32(dEntry.ClusterHigh)<<16)+uint32(dEntry.ClusterLow))
				sort.Slice(fat32LL, func(i, j int) bool {
					return fat32LL[i] < fat32LL[j]
				})
				if err != nil {
					return err
				}
				err = cleanFileContent(driver, fat32LL)
				if err != nil {
					return err
				}
				err = rmFAT32Link(driver, fat32LL)
				if err != nil {
					return err
				}
				err = rmDEntry(driver, offset)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// rmDEntry 将目录项标记为已删除
func rmDEntry(driver *DefaultDriver, dEntryOffset *DirEntryOffset) error {
	sectorNum := dEntryOffset.ClusterNumber*uint32(driver.BPRSector.SectorsPerCluster) + driver.Offset.Data
	buf, err := driver.ReadSector(uint64(sectorNum), 1)
	if err != nil {
		return err
	}
	buf[dEntryOffset.Offset] = 0xe5
	fmt.Println(hex.EncodeToString(buf))
	//err = driver.WriteData(buf, uint64(sectorNum), 0)
	//if err != nil {
	//	return err
	//}
	return nil
}

// rmFAT32Link 删除指定的fat32链
func rmFAT32Link(driver *DefaultDriver, fat32LL []uint32) error {
	sectorNum := fat32LL[0]/128 + uint32(driver.Offset.DEntry)
	buf, err := driver.ReadSector(uint64(sectorNum), 1)
	if err != nil {
		return err
	}
	for _, i := range fat32LL {
		if i == 0x0fffffff {
			//err = driver.WriteData(buf, uint64(sectorNum), 0)
			//if err != nil {
			//	return err
			//}
			fmt.Println(hex.EncodeToString(buf))
			break
		}
		if sectorNum != i/128+uint32(driver.Offset.DEntry) {
			//err = driver.WriteData(buf, uint64(sectorNum), 0)
			//if err != nil {
			//	return err
			//}
			fmt.Println(hex.EncodeToString(buf))
			sectorNum = i/128 + uint32(driver.Offset.DEntry)
			buf, err = driver.ReadSector(uint64(sectorNum), 1)
			if err != nil {
				return err
			}
		}
		offset := (i % 128) * 4
		copy(buf[offset:], []byte{0, 0, 0, 0})
	}
	return nil
}

// cleanFileContent 依据fat32表簇号链清空文件内容
func cleanFileContent(driver *DefaultDriver, fat32LL []uint32) error {
	//buf := make([]byte, 512)
	for _, i := range fat32LL {
		if i == 0x0fffffff {
			break
		}
		for j := 0; j < 8; j++ {
			//fmt.Println(driver.Offset.Data + (i-2)*uint32(driver.BPRSector.SectorsPerCluster) + uint32(j))
			//err := driver.WriteData(buf, uint64(i*uint32(driver.BPRSector.SectorsPerCluster)+uint32(j)), 0)
			//if err != nil {
			//	return err
			//}
		}
	}
	return nil
}

// readFATEntry 读取某号fat表项指向的fat表项
func readFATEntry(driver *DefaultDriver, FATEntry uint32) (uint32, error) {
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
func getFATLink(driver *DefaultDriver, FATEntry uint32) ([]uint32, error) {

	i, err := readFATEntry(driver, FATEntry)
	if err != nil {
		return nil, err
	}
	res := []uint32{FATEntry, i}
	for i != 0x0fffffff {
		i, err = readFATEntry(driver, i)
		if err != nil {
			return nil, err
		}
		res = append(res, i)
	}
	return res, nil
}

// getDriveFactory driver工厂函数，返回driver实例
func getDriveFactory(absFileName string) (*DefaultDriver, error) {
	var driver DefaultDriver
	err := driver.DInit(absFileName)
	if err != nil {
		return nil, err
	}
	return &driver, nil
}

// UpdateFAT 更新fat32表缓冲区
func UpdateFAT(driver *DefaultDriver, fatOffset uint32) error {
	buffer, err := driver.ReadSector(uint64(driver.Offset.DEntry)+uint64(fatOffset), FAT32BufferSize)
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
	driver, err := getDriveFactory(absFileName)
	if err != nil {
		return err
	}

	// 删除目录情况
	var delFileList []string
	stat, err := os.Stat(absFileName)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		delFileList, err = listFiles(absFileName)
		if err != nil {
			return err
		}
	} else {
		delFileList = append(delFileList, absFileName)
	}

	for _, fileName := range delFileList {
		trimPath := strings.TrimPrefix(fileName, driver.Prefix+Segment)
		fmt.Println(trimPath)
		dEntry, dEntryOffset, err := getDirEntry(driver, trimPath)
		if err != nil {
			return err
		}
		err = doRemoveFile(driver, dEntry, dEntryOffset)
		if err != nil {
			return err
		}
	}

	return driver.DDestroy()
}
