package main

// FAT32BootSector 结构体，表示FAT32文件系统的引导扇区信息
type FAT32BootSector struct {
	JumpInstruction       [3]byte   // 0x00~0x02：跳转指令
	OSVersion             [8]byte   // 0x03~0x0A：文件系统标志和版本号
	BytesPerSector        uint16    // 0x0B~0x0C：每扇区字节数
	SectorsPerCluster     uint8     // 0x0D：每簇扇区数
	ReservedSectors       uint16    // 0x0E~0x0F：保留扇区数
	NumFATs               byte      // 0x10：FAT表个数
	MaxRootDirEntries     uint16    // 0x11~0x12：根目录的目录项数（FAT12/16）
	TotalSectors16        uint16    // 0x13~0x14：扇区总数（小于32MB）
	MediaDescriptor       byte      // 0x15：介质描述符
	SectorsPerFAT16       uint16    // 0x16~0x17：每个FAT表的大小扇区数（FAT12/16）
	SectorsPerTrack       uint16    // 0x18~0x19：每磁道扇区数
	NumHeads              uint16    // 0x1A~0x1B：磁头数
	HiddenSectors         uint32    // 0x1C~0x1F：分区前已使用扇区数
	TotalSectors32        uint32    // 0x20~0x23：文件系统大小扇区数（用于超过32MB的情况）
	SectorsPerFAT32       uint32    // 0x24~0x27：每个FAT表的大小扇区数（FAT32）
	Flags                 uint16    // 0x28~0x29：标记
	Version               uint16    // 0x2A~0x2B：版本号
	RootCluster           uint32    // 0x2C~0x2F：根目录簇号
	FSInfoSector          uint16    // 0x30~0x31：FSINFO扇区号
	BackupBootSector      uint16    // 0x32~0x33：备份引导扇区位置
	Reserved              [12]byte  // 0x34~0x3F：未使用
	BIOSDriveNum          byte      // 0x40：BIOS INT 13H 设备号
	Unused                byte      // 0x41：未用
	ExtendedBootSignature byte      // 0x42：扩展引导标志
	VolumeSerialNumber    uint32    // 0x43~0x46：卷序列号
	VolumeLabel           [11]byte  // 0x47~0x51：卷标（ASCII）
	FileSystemType        [8]byte   // 0x52~0x59：文件系统格式（ASCII），如FAT32
	Unused2               [410]byte // 0x5A~0x1FD：未使用
	Signature             uint16    // 0x1FE~0x1FF：签名标志“55 AA”
}

type FAT32DirEntry struct {
	FileName         [11]byte // 0x00~0x0A：文件名（ASCII）
	FileAttributes   uint8    // 0x0B：文件属性
	Reserved         uint8    // 0x0C：保留
	CreateTimeFine   uint8    // 0x0D：建立时间（精确到0.01秒）
	CreateTime       uint16   // 0x0E~0x0F：建立时间
	CreateDate       uint16   // 0x10~0x11：建立日期
	LastAccessDate   uint16   // 0x12~0x13：最后访问日期
	ClusterHigh      uint16   // 0x14~0x15：文件起始簇号的高位
	LastModifiedTime uint16   // 0x16~0x17：最后修改时间
	LastModifiedDate uint16   // 0x18~0x19：最后修改日期
	ClusterLow       uint16   // 0x1A~0x1B：文件起始簇号的低位
	FileSize         uint32   // 0x1C~0x1F：文件大小（字节）
}

type FAT32LongDirEntry struct {
	SequenceNumber byte      // 序列号，第一个长文件名序列号为1，或最后一个序号 | 0x40
	Name1          [5]uint16 // 长文件名的第1~5个字符，每个字符2字节
	Attribute      byte      // 属性标志，一定是0x0F
	Reserved1      byte      // 保留字节
	Checksum       byte      // 校验和，同一文件的长文件名目录项具有相同校验和
	Name2          [6]uint16 // 长文件名的第6~11个字符，每个字符2字节
	Reserved2      [2]byte   // 保留字节
	Name3          [2]uint16 // 长文件名的第12~13个字符，每个字符2字节
}

type FAT32Offset struct {
	DEntry uint   // FAT表区
	Data   uint32 // 数据区
}

type FAT32Buffer struct {
	// 存储32扇区的fat32表，占用内存 32 * 512 = 16k
	Number uint32
	Link   []uint32
}

type DirEntryBuffer struct {
	// 存储32个扇区的目录表，可用 16 * 32 = 512 个目录项， 占用内存 32 * 512 = 16k
	Number uint32
	DEntry []*FAT32DirEntry
}

// Driver 抽象驱动器结构，linux与win分别实现
type Driver interface {
	DInit(absFileName string) error
	ReadSector(sectorNum uint64, readNum uint16) (buffer []byte, err error)
	DDestroy() error
}
