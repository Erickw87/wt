package codec

// 数据压缩辅助（对应 WTSCmpHelper.hpp 使用的 ZSTD）

import (
	"github.com/klauspost/compress/zstd"
)

var (
	zstdEnc, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	zstdDec, _ = zstd.NewReader(nil)
)

// CompressZstd 压缩数据（对应 WTSCmpHelper::compress_data）
func CompressZstd(src []byte) []byte {
	return zstdEnc.EncodeAll(src, make([]byte, 0, len(src)/2))
}

// DecompressZstd 解压数据（对应 WTSCmpHelper::uncompress_data）
func DecompressZstd(src []byte) ([]byte, error) {
	out, err := zstdDec.DecodeAll(src, nil)
	if err != nil {
		return nil, err
	}
	return out, nil
}