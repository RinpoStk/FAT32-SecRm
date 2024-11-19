//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"golang.org/x/sys/windows"
	"log"
	"path/filepath"
)

const (
	FSCTL_LOCK_VOLUME   = 0x00090018 // FSCTL_LOCK_VOLUME 锁定卷的控制码
	FSCTL_UNLOCK_VOLUME = 0x0009001C // FSCTL_UNLOCK_VOLUME 解锁卷的控制码
	Segment             = `\`        // Segment 系统使用路径分隔符
)

type DefaultDriver struct {
	Handle    windows.Handle
	Prefix    string
	BPRSector *FAT32BootSector
	Offset    *FAT32Offset
}

func (d *DefaultDriver) DInit(absFileName string) error {
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

func (d *DefaultDriver) ReadSector(sectorNum uint64, readNum uint16) ([]byte, error) {
	var bytesRead uint32
	bufferSize := d.BPRSector.BytesPerSector * readNum
	buffer := make([]byte, bufferSize)

	// 修改句柄偏移
	offsetByte := uint64(d.BPRSector.BytesPerSector) * sectorNum
	high := int32(offsetByte >> 32)
	low := int32(offsetByte & 0xFFFFFFFF)
	_, err := windows.SetFilePointer(
		d.Handle,
		low,
		&high,
		windows.FILE_BEGIN,
	)
	if err != nil {
		return nil, err
	}
	// 读取扇区
	err = windows.ReadFile(d.Handle, buffer, &bytesRead, nil)
	if err != nil {
		return nil, err
	}
	return buffer[:bytesRead], nil
}

func (d *DefaultDriver) WriteData(data []byte, sectorNum uint64, offset uint16) error {
	err := lockVolume(d.Handle)
	if err != nil {
		return err
	}
	// 确保解锁卷
	defer func() {
		if err = unlockVolume(d.Handle); err != nil {
			log.Fatal(err)
		}
	}()

	var buf []byte
	if len(data) < int(d.BPRSector.BytesPerSector) {
		// 创建写入缓冲区
		buf, err = d.ReadSector(sectorNum, 1)
		if err != nil {
			return err
		}
		copy(buf[offset:], data)
	} else if len(data) > int(d.BPRSector.BytesPerSector) {
		return errors.New("data len longer than one sector")
	} else {
		buf = data
	}

	offsetByte := int64(d.BPRSector.BytesPerSector)*int64(sectorNum) + int64(offset)
	high := int32(offsetByte >> 32)
	low := int32(offsetByte & 0xFFFFFFFF)
	_, err = windows.SetFilePointer(
		d.Handle,
		low,
		&high,
		windows.FILE_BEGIN,
	)
	if err != nil {
		return err
	}
	var written uint32

	err = windows.WriteFile(
		d.Handle,
		buf,
		&written,
		nil,
	)
	if err != nil {
		return err
	}
	return nil
}

func (d *DefaultDriver) DDestroy() error {
	return windows.CloseHandle(d.Handle)
}

// openPartition 打开逻辑分区 示例 openPartition(`D:`)
func openPartition(partitionName string) (windows.Handle, error) {
	// 创建句柄
	partitionHandle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(`\\.\`+partitionName),
		windows.GENERIC_ALL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)

	if err != nil {
		return 0, err
	}
	return partitionHandle, nil
}

// getBPR 读取FAT32引导扇区(BPR)
func getBPR(handle windows.Handle) (*FAT32BootSector, error) {
	var bytesRead uint32
	var fat32BootSector FAT32BootSector
	var buffer [512]byte
	// 读取分卷的前512字节，即BPR，并解析到BPRSector中
	err := windows.ReadFile(handle, (&buffer)[:], &bytesRead, nil)
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

func lockVolume(handle windows.Handle) error {
	var bytesReturned uint32
	err := windows.DeviceIoControl(
		handle,
		FSCTL_LOCK_VOLUME,
		nil,
		0,
		nil,
		0,
		&bytesReturned,
		nil,
	)
	if err != nil {
		return err
	}
	return nil
}

func unlockVolume(handle windows.Handle) error {
	var bytesReturned uint32
	err := windows.DeviceIoControl(
		handle,
		FSCTL_UNLOCK_VOLUME, // 解锁磁盘的控制代码
		nil,
		0,
		nil,
		0,
		&bytesReturned,
		nil,
	)
	if err != nil {
		return err
	}
	return nil
}
