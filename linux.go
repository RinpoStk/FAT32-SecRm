//go:build linux

package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const Segment = `/`

type DefalutDriver struct {
	Fd        int
	Prefix    string
	BPRSector *FAT32BootSector
	Offset    *FAT32Offset
}

func (d *DefalutDriver) DInit(absFileName string) error {
	device, mountPoint, err := getMount(absFileName)
	if err != nil {
		return err
	}
	d.Prefix = mountPoint
	fd, err := openFd(device)
	if err != nil {
		return err
	}
	d.Fd = fd

	d.BPRSector, err = getBPR(d.Fd)
	if err != nil {
		return err
	}
	// 初始化计算重要偏移处
	d.Offset = &FAT32Offset{}
	d.Offset.DEntry = uint(d.BPRSector.ReservedSectors)
	d.Offset.Data = uint32(d.Offset.DEntry) + 2*uint32(d.BPRSector.SectorsPerFAT32)
	return UpdateFAT(d, 0)
}

func (d *DefalutDriver) ReadSector(sectorNum uint64, readNum uint16) ([]byte, error) {
	bufferSize := d.BPRSector.BytesPerSector * readNum

	buffer := make([]byte, bufferSize)

	// 修改文件描述符偏移
	offsetByte := int64(d.BPRSector.BytesPerSector) * int64(sectorNum)

	// 读取扇区
	bytesRead, err := syscall.Pread(d.Fd, buffer, offsetByte)
	if err != nil {
		return nil, err
	}
	return buffer[:bytesRead], nil
}

func (d *DefalutDriver) WriteData(data []byte, sectorNum uint64, offset uint16) error {
	offsetByte := int64(offset) * int64(sectorNum) * int64(d.BPRSector.BytesPerSector)
	_, err := syscall.Pwrite(d.Fd, data, offsetByte)
	if err != nil {
		return err
	}
	return nil
}

func (d *DefalutDriver) DDestroy() error {
	return syscall.Close(d.Fd)
}

// getMount 获取挂载点，驱动器名
func getMount(absFileName string) (string, string, error) {
	// 解析挂载点
	mounts, err := os.Open("/proc/mounts")
	if err != nil {
		return "", "", err
	}
	var mountPoints []string
	var devices []string
	// 逐行读取 /proc/mounts 并解析挂载点
	scanner := bufio.NewScanner(mounts)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			mountPoints = append(mountPoints, fields[1])
			devices = append(devices, fields[0])
		}
	}
	if err = scanner.Err(); err != nil {
		return "", "", err
	}
	// 遍历找到最深的挂载点
	var bestMatch string
	var index int
	for i, mountPoint := range mountPoints {
		// 检查 mountPoint 是否是 path 的前缀，且是路径的边界
		if strings.HasPrefix(absFileName, mountPoint) && len(mountPoint) > len(bestMatch) {
			// 检查是否为父目录
			relPath, err := filepath.Rel(mountPoint, absFileName)
			if err != nil {
				return "", "", err
			}
			if !strings.HasPrefix(relPath, "..") {
				bestMatch = mountPoint
				index = i
			}
		}
	}
	if bestMatch == "" {
		return "", "", errors.New("no mount point found")
	}
	err = mounts.Close()
	if err != nil {
		return "", "", err
	}
	return devices[index], bestMatch, nil
}

func openFd(mountPoint string) (int, error) {
	fd, err := syscall.Open(mountPoint, syscall.O_RDONLY, 0)
	if err != nil {
		return -1, err
	}
	return fd, nil
}

func getBPR(fd int) (*FAT32BootSector, error) {
	buffer := make([]byte, 512)
	// 读取分卷的前512字节，即BPR，并解析到BPRSector中
	bytesRead, err := syscall.Read(fd, buffer)
	if err != nil {
		return nil, err
	}
	if bytesRead != 512 {
		return nil, errors.New("read sector error")
	}
	var fat32BootSector FAT32BootSector
	err = binary.Read(bytes.NewReader(buffer[:bytesRead]), binary.LittleEndian, &fat32BootSector)
	if err != nil {
		return nil, err
	}
	return &fat32BootSector, nil
}
