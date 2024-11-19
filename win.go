//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"path/filepath"
	"syscall"
)

// Segment 系统使用路径分隔符
const Segment = `\`

type DefalutDriver struct {
	Handle    syscall.Handle
	Prefix    string
	BPRSector *FAT32BootSector
	Offset    *FAT32Offset
}

func (d *DefalutDriver) DInit(absFileName string) error {
	var err error
	volName := filepath.VolumeName(absFileName)
	d.Handle, err = openPartition(volName)
	d.Prefix = volName

	if err != nil {
		return err
	}
	d.BPRSector, err = getBPR(d.Handle)
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
	var bytesRead uint32
	bufferSize := d.BPRSector.BytesPerSector * readNum
	buffer := make([]byte, bufferSize)

	// 修改句柄偏移
	offsetByte := uint64(d.BPRSector.BytesPerSector) * sectorNum
	high := int32(offsetByte >> 32)
	low := int32(offsetByte & 0xFFFFFFFF)
	_, err := syscall.SetFilePointer(
		d.Handle,
		low,
		&high,
		syscall.FILE_BEGIN,
	)
	if err != nil {
		return nil, err
	}
	// 读取扇区
	err = syscall.ReadFile(d.Handle, buffer, &bytesRead, nil)
	if err != nil {
		return nil, err
	}
	return buffer[:bytesRead], nil
}

func (d *DefalutDriver) DDestroy() error {
	return syscall.CloseHandle(d.Handle)
}

// openPartition 打开逻辑分区 示例 openPartition(`D:`)
func openPartition(partitionName string) (syscall.Handle, error) {
	// 创建句柄
	partitionHandle, err := syscall.CreateFile(
		syscall.StringToUTF16Ptr(`\\.\`+partitionName),
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		0,
		0,
	)

	if err != nil {
		return 0, err
	}

	return partitionHandle, nil
}

// getBPR 读取FAT32引导扇区(BPR)
func getBPR(handle syscall.Handle) (*FAT32BootSector, error) {
	var bytesRead uint32
	var fat32BootSector FAT32BootSector
	var buffer [512]byte
	// 读取分卷的前512字节，即BPR，并解析到BPRSector中
	err := syscall.ReadFile(handle, (&buffer)[:], &bytesRead, nil)
	if err != nil {
		return nil, err
	}
	if bytesRead != 512 {
		return nil, errors.New("read sector error")
	}
	err = binary.Read(bytes.NewReader(buffer[:bytesRead]), binary.LittleEndian, &fat32BootSector)
	if err != nil {
		return nil, err
	}
	return &fat32BootSector, nil
}
