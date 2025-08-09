package writer

import (
	"bytes"
	"encoding/binary"
	"path/filepath"

	"wtdata/internal/types"
)

func rtOrdQueSize() int { return 280 }
func rtOrdDtlSize() int { return 88 }
func rtTransSize() int { return 104 }

// WriteOrderQueue（对应 WtDataWriter::procQueue 追加行为）
func (w *Writer) WriteOrderQueue(exchg string, code string, date uint32, que *types.WTSOrdQueStruct) error {
	if w.disableOrdQue { return nil }
	dir := filepath.Join(w.baseDir, "rt", "queue", exchg)
	f, mapped, _, _, err := w.openMapped(dir, exchg, code, uint16(types.BT_RT_OrdQueue), date, rtOrdQueSize(), 2500)
	if err != nil { return err }
	defer func(){ _ = f.Close() }()
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, que)
	m2, err := w.appendOne(f, mapped, buf.Bytes(), rtOrdQueSize())
	if err != nil { return err }
	_ = m2
	return nil
}

// WriteOrderDetail（对应 WtDataWriter::procOrder 追加行为）
func (w *Writer) WriteOrderDetail(exchg string, code string, date uint32, od *types.WTSOrdDtlStruct) error {
	if w.disableOrdDtl { return nil }
	dir := filepath.Join(w.baseDir, "rt", "orders", exchg)
	f, mapped, _, _, err := w.openMapped(dir, exchg, code, uint16(types.BT_RT_OrdDetail), date, rtOrdDtlSize(), 2500)
	if err != nil { return err }
	defer func(){ _ = f.Close() }()
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, od)
	m2, err := w.appendOne(f, mapped, buf.Bytes(), rtOrdDtlSize())
	if err != nil { return err }
	_ = m2
	return nil
}

// WriteTransaction（对应 WtDataWriter::procTrans 追加行为）
func (w *Writer) WriteTransaction(exchg string, code string, date uint32, tr *types.WTSTransStruct) error {
	if w.disableTrans { return nil }
	dir := filepath.Join(w.baseDir, "rt", "trans", exchg)
	f, mapped, _, _, err := w.openMapped(dir, exchg, code, uint16(types.BT_RT_Trnsctn), date, rtTransSize(), 2500)
	if err != nil { return err }
	defer func(){ _ = f.Close() }()
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, tr)
	m2, err := w.appendOne(f, mapped, buf.Bytes(), rtTransSize())
	if err != nil { return err }
	_ = m2
	return nil
}