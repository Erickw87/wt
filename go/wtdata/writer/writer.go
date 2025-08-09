package writer

// 数据写入（对应 WtDataWriter）

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"wtdata/internal/codec"
	"wtdata/internal/types"
)

// Writer（对应 WtDataWriter，简化版本：单线程重写文件，不做mmap/扩容映射）
type Writer struct {
	baseDir       string
	saveTickCSV   bool
	disableHis    bool
	disableTick   bool
	disableMin1   bool
	disableMin5   bool
	disableDay    bool
	disableTrans  bool
	disableOrdQue bool
	disableOrdDtl bool
}

func (w *Writer) Init(base string) {
	if base != "" && base[len(base)-1] != '/' {
		base += "/"
	}
	w.baseDir = base
}

// WriteTick 写入一条tick到 rt 文件（对应 WtDataWriter::pipeToTicks），简化：整文件重写追加
func (w *Writer) WriteTick(exchg string, code string, date uint32, tick *types.WTSTickStruct) error {
	if w.disableTick {
		return nil
	}
	dir := filepath.Join(w.baseDir, "rt", "ticks", exchg)
	_ = os.MkdirAll(dir, 0o755)
	fn := filepath.Join(dir, fmt.Sprintf("%s.dmb", code))
	payload := []byte{}
	if b, err := os.ReadFile(fn); err == nil && len(b) >= 24 {
		payload = b[24:]
	}
	// 追加一个 tick
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, tick)
	payload = append(payload, buf.Bytes()...)
	// 重写 header + payload
	head := make([]byte, 24)
	copy(head[0:8], []byte{'&', '^', '%', '$', '#', '@', '!', 0})
	binary.LittleEndian.PutUint16(head[8:10], uint16(types.BT_RT_Ticks))
	binary.LittleEndian.PutUint16(head[10:12], uint16(types.BLOCK_VERSION_RAW_V2))
	size := uint32(len(payload) / rtTickSize())
	binary.LittleEndian.PutUint32(head[12:16], size)
	binary.LittleEndian.PutUint32(head[16:20], size)
	binary.LittleEndian.PutUint32(head[20:24], date)
	return os.WriteFile(fn, append(head, payload...), 0o644)
}

func rtTickSize() int { return 512 }

// CloseToHis 将 rt 数据转存为历史 .dsb（对应 WtDataWriter::proc_loop 的转存逻辑，简化版）
func (w *Writer) CloseToHis(exchg string, code string, date uint32) error {
	if w.disableHis {
		return nil
	}
	// ticks
	if !w.disableTick {
		fnRt := filepath.Join(w.baseDir, "rt", "ticks", exchg, fmt.Sprintf("%s.dmb", code))
		b, err := os.ReadFile(fnRt)
		if err == nil && len(b) >= 24 {
			payload := b[24:]
			cmp := codec.CompressZstd(payload)
			header := make([]byte, 20)
			copy(header[0:8], []byte{'&', '^', '%', '$', '#', '@', '!', 0})
			binary.LittleEndian.PutUint16(header[8:10], uint16(types.BT_HIS_Ticks))
			binary.LittleEndian.PutUint16(header[10:12], uint16(types.BLOCK_VERSION_CMP_V2))
			binary.LittleEndian.PutUint64(header[12:20], uint64(len(cmp)))
			outDir := filepath.Join(w.baseDir, "his", "ticks", exchg, fmt.Sprintf("%d", date))
			_ = os.MkdirAll(outDir, 0o755)
			fn := filepath.Join(outDir, fmt.Sprintf("%s.dsb", code))
			if err := os.WriteFile(fn, append(header, cmp...), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// 其他写入（逐笔/队列/K线）可按相同模式补齐；完整一致性需要 mmap 与容量管理，这里为功能基线。

var ErrNotImplemented = errors.New("not implemented")