package writer

import (
	"bytes"
	"encoding/binary"
	"path/filepath"

	"wtdata/internal/types"
)

func rtBarSize() int { return 88 }

// WriteMin1 追加 1 分钟K线（对应 WtDataWriter::pipeToKlines 中的 min1 落盘语义，写 rt/min1/{exchg}/{code}.dmb）
func (w *Writer) WriteMin1(exchg, code string, date uint32, bar *types.WTSBarStruct) error {
	if w.disableMin1 { return nil }
	dir := filepath.Join(w.baseDir, "rt", "min1", exchg)
	f, mapped, _, _, err := w.openMapped(dir, exchg, code, uint16(types.BT_RT_Minute1), date, rtBarSize(), barInitCap())
	if err != nil { return err }
	defer func(){ _ = f.Close() }()
	m2, err := w.appendOne(f, mapped, packBar(bar), rtBarSize())
	if err != nil { return err }
	_ = m2
	return nil
}

// WriteMin5 追加 5 分钟K线（对应 WtDataWriter::pipeToKlines 中的 min5 落盘语义，写 rt/min5/{exchg}/{code}.dmb）
func (w *Writer) WriteMin5(exchg, code string, date uint32, bar *types.WTSBarStruct) error {
	if w.disableMin5 { return nil }
	dir := filepath.Join(w.baseDir, "rt", "min5", exchg)
	f, mapped, _, _, err := w.openMapped(dir, exchg, code, uint16(types.BT_RT_Minute5), date, rtBarSize(), barInitCap()/5)
	if err != nil { return err }
	defer func(){ _ = f.Close() }()
	m2, err := w.appendOne(f, mapped, packBar(bar), rtBarSize())
	if err != nil { return err }
	_ = m2
	return nil
}

func barInitCap() int { return 24 * 60 } // 预分配分钟数

func packBar(bar *types.WTSBarStruct) []byte {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, bar)
	return buf.Bytes()
}