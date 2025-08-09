package codec

// 存储块处理（对应 WtDataReader/WtDataWriter 中的 proc_block_data）

import (
	"encoding/binary"
	"errors"
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
// isBar 用于旧版结构转换（当前先不实现旧版转换，如需兼容可后续扩展）。
func ProcBlockData(content []byte, isBar bool, keepHead bool) ([]byte, error) {
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
				// 旧版压缩头（V1），C++ 会直接取头后数据并解压；但 V1 格式未携带压缩大小，需要协议约定。
				// 本实现聚焦 V2 路径，V1 支持将后续补齐。
				return nil, errors.New("BLOCK_VERSION_CMP (V1) not supported yet")
			}
			if h.Ver == BLOCK_VERSION_RAW {
				// 旧版未压缩且旧结构体，C++ 会直接去掉头并做老到新结构转换。
				// 先直接去头返回，转换逻辑留待调用端按需要处理。
				if keepHead {
					return content, nil
				}
				return content[12:], nil
			}
		}
	}

	// 无法识别
	return nil, errors.New("unknown block header or version")
}