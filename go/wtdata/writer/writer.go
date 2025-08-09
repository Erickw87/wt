package writer

// 数据写入（对应 WtDataWriter）

import (
	"bytes"
	"encoding/binary"
	"os"
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

	// tick cache
	tcFile *os.File
	tcMap  []byte
	tcIdx  map[string]uint32
	tcSize uint32
	tcCap  uint32
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
	// 更新 TickCache
	_ = w.UpdateTickCache(exchg, code, date, tick)
	return nil
}

func rtTickSize() int { return 512 }