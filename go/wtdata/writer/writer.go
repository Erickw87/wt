package writer

// 数据写入（对应 WtDataWriter）

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"

	"wtdata/internal/types"
)

// Writer（对应 WtDataWriter，采用 mmap 追加+扩容，避免整文件重写）
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

// WriteTick 写入一条tick到 rt 文件（对应 WtDataWriter::pipeToTicks）
func (w *Writer) WriteTick(exchg string, code string, date uint32, tick *types.WTSTickStruct) error {
	if w.disableTick { return nil }
	dir := filepath.Join(w.baseDir, "rt", "ticks", exchg)
	f, mapped, _, _, err := w.openMapped(dir, exchg, code, uint16(types.BT_RT_Ticks), date, rtTickSize(), 2500)
	if err != nil { return err }
	defer func(){ _ = f.Close() }()
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, tick)
	m2, err := w.appendOne(f, mapped, buf.Bytes(), rtTickSize())
	if err != nil { return err }
	_ = m2 // keep mapped alive until function returns
	return nil
}

func rtTickSize() int { return 512 }

// CloseToHis 将 rt 数据转存为历史 .dsb（对应 WtDataWriter::proc_loop 的转存逻辑，后续文件压缩在其他单元实现）
func (w *Writer) CloseToHis(exchg string, code string, date uint32) error {
	// 留空，后续实现完整转存（tick/trans/orddtl/ordque/min1/min5/day）
	fmt.Printf("CloseToHis pending for %s.%s on %d\n", exchg, code, date)
	return nil
}