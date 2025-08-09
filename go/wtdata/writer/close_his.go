package writer

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"wtdata/internal/codec"
	"wtdata/internal/types"
)

// CloseToHis 将 rt 数据转存为历史 .dsb（对应 WtDataWriter::proc_loop 的转存顺序，简化：先实现 ticks/trans/orddtl/ordque/min1/min5）
func (w *Writer) CloseToHis(exchg string, code string, date uint32) error {
	if !w.disableTick {
		if err := w.dumpRTPayload(exchg, code, date, "ticks", types.BT_HIS_Ticks, rtTickSize()); err != nil { return err }
	}
	if !w.disableTrans {
		if err := w.dumpRTPayload(exchg, code, date, "trans", types.BT_HIS_Trnsctn, rtTransSize()); err != nil { return err }
	}
	if !w.disableOrdDtl {
		if err := w.dumpRTPayload(exchg, code, date, "orders", types.BT_HIS_OrdDetail, rtOrdDtlSize()); err != nil { return err }
	}
	if !w.disableOrdQue {
		if err := w.dumpRTPayload(exchg, code, date, "queue", types.BT_HIS_OrdQueue, rtOrdQueSize()); err != nil { return err }
	}
	if !w.disableMin1 {
		if err := w.dumpRTBars(exchg, code, "min1", types.BT_HIS_Minute1); err != nil { return err }
	}
	if !w.disableMin5 {
		if err := w.dumpRTBars(exchg, code, "min5", types.BT_HIS_Minute5); err != nil { return err }
	}
	return nil
}

func (w *Writer) dumpRTPayload(exchg, code string, date uint32, subdir string, hisType uint16, elemSize int) error {
	rtPath := filepath.Join(w.baseDir, "rt", subdir, exchg, fmt.Sprintf("%s.dmb", code))
	b, err := os.ReadFile(rtPath)
	if err != nil || len(b) < 24 { return nil }
	payload := b[24:]
	cmp := codec.CompressZstd(payload)
	head := make([]byte, 20)
	copy(head[0:8], []byte{'&','^','%','$','#','@','!',0})
	binary.LittleEndian.PutUint16(head[8:10], hisType)
	binary.LittleEndian.PutUint16(head[10:12], uint16(types.BLOCK_VERSION_CMP_V2))
	binary.LittleEndian.PutUint64(head[12:20], uint64(len(cmp)))
	outDir := filepath.Join(w.baseDir, "his", subdir, exchg, fmt.Sprintf("%d", date))
	_ = os.MkdirAll(outDir, 0o755)
	fn := filepath.Join(outDir, fmt.Sprintf("%s.dsb", code))
	return os.WriteFile(fn, append(head, cmp...), 0o644)
}

func (w *Writer) dumpRTBars(exchg, code, subdir string, hisType uint16) error {
	rtPath := filepath.Join(w.baseDir, "rt", subdir, exchg, fmt.Sprintf("%s.dmb", code))
	b, err := os.ReadFile(rtPath)
	if err != nil || len(b) < 24 { return nil }
	payload := b[24:]
	cmp := codec.CompressZstd(payload)
	head := make([]byte, 20)
	copy(head[0:8], []byte{'&','^','%','$','#','@','!',0})
	binary.LittleEndian.PutUint16(head[8:10], hisType)
	binary.LittleEndian.PutUint16(head[10:12], uint16(types.BLOCK_VERSION_CMP_V2))
	binary.LittleEndian.PutUint64(head[12:20], uint64(len(cmp)))
	outDir := filepath.Join(w.baseDir, "his", subdir, exchg)
	_ = os.MkdirAll(outDir, 0o755)
	fn := filepath.Join(outDir, fmt.Sprintf("%s.dsb", code))
	return os.WriteFile(fn, append(head, cmp...), 0o644)
}