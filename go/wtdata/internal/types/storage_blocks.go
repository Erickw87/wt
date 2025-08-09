package types

// 存储块头与数据块结构（对应 src/WtDataStorage/DataDefine.h）

// BlockType（对应 DataDefine.h BlockType）
const (
	BT_RT_Minute1   = 1
	BT_RT_Minute5   = 2
	BT_RT_Ticks     = 3
	BT_RT_Cache     = 4
	BT_RT_Trnsctn   = 5
	BT_RT_OrdDetail = 6
	BT_RT_OrdQueue  = 7

	BT_HIS_Minute1   = 21
	BT_HIS_Minute5   = 22
	BT_HIS_Day       = 23
	BT_HIS_Ticks     = 24
	BT_HIS_Trnsctn   = 25
	BT_HIS_OrdDetail = 26
	BT_HIS_OrdQueue  = 27
)

// BLOCK_VERSION_*（对应 DataDefine.h 版本常量）
const (
	BLOCK_VERSION_RAW     = 0x01
	BLOCK_VERSION_CMP     = 0x02
	BLOCK_VERSION_RAW_V2  = 0x03
	BLOCK_VERSION_CMP_V2  = 0x04
)

// BLK_FLAG（对应 DataDefine.h BLK_FLAG）
var BLK_FLAG = [8]byte{'&', '^', '%', '$', '#', '@', '!', 0}

// BlockHeader（对应 DataDefine.h BlockHeader，pack(1)）
type BlockHeader struct {
	BlkFlag  [8]byte
	Type     uint16
	Version  uint16
}

// BlockHeaderV2（对应 DataDefine.h BlockHeaderV2，pack(1)）
type BlockHeaderV2 struct {
	BlkFlag [8]byte
	Type    uint16
	Version uint16
	Size    uint64 // 压缩后数据大小
}

// RTBlockHeader（对应 DataDefine.h RTBlockHeader，pack(1)）
type RTBlockHeader struct {
	BlockHeader
	Size     uint32
	Capacity uint32
}

// RTDayBlockHeader（对应 DataDefine.h RTDayBlockHeader，pack(1)）
type RTDayBlockHeader struct {
	RTBlockHeader
	Date uint32
}

// 实时块（对应 DataDefine.h 各 RT*Block）
type RTKlineBlock struct {
	RTDayBlockHeader
	Bars []WTSBarStruct // 尾随数组语义，Go 用切片承载，读写通过编解码处理
}

type RTTickBlock struct {
	RTDayBlockHeader
	Ticks []WTSTickStruct
}

type RTTransBlock struct {
	RTDayBlockHeader
	Trans []WTSTransStruct
}

type RTOrdDtlBlock struct {
	RTDayBlockHeader
	Details []WTSOrdDtlStruct
}

type RTOrdQueBlock struct {
	RTDayBlockHeader
	Queues []WTSOrdQueStruct
}

// TickCache（对应 DataDefine.h RTTickCache/TickCacheItem）
type TickCacheItem struct {
	Date uint32
	Tick WTSTickStruct
}

type RTTickCache struct {
	RTBlockHeader
	Ticks []TickCacheItem
}