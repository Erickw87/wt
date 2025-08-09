package codec

// 存储块处理（对应 WtDataReader/WtDataWriter 中的 proc_block_data）

import (
	"encoding/binary"
	"errors"

	"wtdata/internal/types"
)

// Block header layouts（对应 DataDefine.h）
type blockHeader struct {
	BlkFlag [8]byte
	Type   uint16
	Ver    uint16
}

type blockHeaderV2 struct {
	BlkFlag [8]byte
	Type    uint16
	Ver     uint16
	Size    uint64
}

var blkMagic = [8]byte{'&', '^', '%', '$', '#', '@', '!', 0}

const (
	BLOCK_VERSION_RAW     = 0x01
	BLOCK_VERSION_CMP     = 0x02
	BLOCK_VERSION_RAW_V2  = 0x03
	BLOCK_VERSION_CMP_V2  = 0x04
)

// ProcBlockData 等价于 C++ 的 proc_block_data，返回净荷数据（可选择是否保留头部）。
// 自动识别 V1/V2；V1 的 bars/ticks 自动转换为新版布局；其他类型按原样返回。
func ProcBlockData(content []byte, _isBar bool, keepHead bool) ([]byte, error) {
	if len(content) < 8+2+2 {
		return nil, errors.New("content too small")
	}
	// 尝试 V2 头
	if len(content) >= int(binary.Size(blockHeaderV2{})) {
		var h2 blockHeaderV2
		copy(h2.BlkFlag[:], content[0:8])
		h2.Type = binary.LittleEndian.Uint16(content[8:10])
		h2.Ver = binary.LittleEndian.Uint16(content[10:12])
		h2.Size = binary.LittleEndian.Uint64(content[12:20])

		if h2.BlkFlag == blkMagic && (h2.Ver == BLOCK_VERSION_CMP_V2) {
			// 压缩帧
			if len(content) != int(20+h2.Size) {
				return nil, errors.New("compressed block size mismatch")
			}
			payload := content[20:]
			ud, err := DecompressZstd(payload)
			if err != nil {
				return nil, err
			}
			if keepHead {
				// 返回一个 RAW_V2 头 + 解压后的数据
				buf := make([]byte, 8+2+2)
				copy(buf[0:8], blkMagic[:])
				binary.LittleEndian.PutUint16(buf[8:10], h2.Type)
				binary.LittleEndian.PutUint16(buf[10:12], BLOCK_VERSION_RAW_V2)
				buf = append(buf, ud...)
				return buf, nil
			}
			return ud, nil
		}
	}

	// 尝试 V1/RAW_V2 头
	if len(content) >= int(binary.Size(blockHeader{})) {
		var h blockHeader
		copy(h.BlkFlag[:], content[0:8])
		h.Type = binary.LittleEndian.Uint16(content[8:10])
		h.Ver = binary.LittleEndian.Uint16(content[10:12])
		if h.BlkFlag == blkMagic {
			if h.Ver == BLOCK_VERSION_RAW_V2 {
				if keepHead {
					return content, nil
				}
				return content[12:], nil
			}
			if h.Ver == BLOCK_VERSION_CMP {
				// 旧版压缩头（V1）：直接对头后面数据解压
				ud, err := DecompressZstd(content[12:])
				if err != nil { return nil, err }
				// bars/ticks 需要从 Old -> New
				newPayload, err := convertIfOld(h.Type, ud)
				if err != nil { return nil, err }
				if keepHead {
					buf := make([]byte, 12)
					copy(buf[0:8], blkMagic[:])
					binary.LittleEndian.PutUint16(buf[8:10], h.Type)
					binary.LittleEndian.PutUint16(buf[10:12], BLOCK_VERSION_RAW_V2)
					buf = append(buf, newPayload...)
					return buf, nil
				}
				return newPayload, nil
			}
			if h.Ver == BLOCK_VERSION_RAW {
				payload := content[12:]
				newPayload, err := convertIfOld(h.Type, payload)
				if err != nil { return nil, err }
				if keepHead {
					buf := make([]byte, 12)
					copy(buf[0:8], blkMagic[:])
					binary.LittleEndian.PutUint16(buf[8:10], h.Type)
					binary.LittleEndian.PutUint16(buf[10:12], BLOCK_VERSION_RAW_V2)
					buf = append(buf, newPayload...)
					return buf, nil
				}
				return newPayload, nil
			}
		}
	}

	// 无法识别
	return nil, errors.New("unknown block header or version")
}

// convertIfOld 将 V1 Old payload 转换成 V2 New；仅对 bars/ticks 生效，其他直接透传
func convertIfOld(blkType uint16, payload []byte) ([]byte, error) {
	switch blkType {
	case types.BT_RT_Minute1, types.BT_RT_Minute5, types.BT_HIS_Minute1, types.BT_HIS_Minute5, types.BT_HIS_Day:
		return BarOldToNew(payload)
	case types.BT_RT_Ticks, types.BT_HIS_Ticks:
		return TickOldToNew(payload)
	default:
		return payload, nil
	}
}