package writer

// 实时文件管理：打开/映射/扩容/追加（对应 WtDataWriter 中 RT 块管理逻辑）

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"wtdata/internal/mm"
)

// rtHeaderLen = 24 字节（见 rt.parse）
const rtHeaderLen = 24

// openMapped 打开（创建）rt 文件并映射返回内存，返回 mapped bytes, header fields(size,capacity,date)
func (w *Writer) openMapped(dir, exchg, code string, blkType uint16, date uint32, elemSize int, initCapacity int) (f *os.File, mapped []byte, size uint32, capacity uint32, err error) {
	_ = os.MkdirAll(dir, 0o755)
	fn := filepath.Join(dir, fmt.Sprintf("%s.dmb", code))
	f, err = os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	st, _ := f.Stat()
	if st.Size() < rtHeaderLen {
		// 初始化
		cap := initCapacity
		sz := rtHeaderLen + cap*elemSize
		if err = mm.EnsureSize(f, int64(sz)); err != nil {
			return
		}
		mapped, err = mm.MapRW(f, sz)
		if err != nil { return }
		// header
		copy(mapped[0:8], []byte{'&','^','%','$','#','@','!',0})
		binary.LittleEndian.PutUint16(mapped[8:10], blkType)
		binary.LittleEndian.PutUint16(mapped[10:12], uint16(3)) // RAW_V2
		binary.LittleEndian.PutUint32(mapped[12:16], 0)         // size
		binary.LittleEndian.PutUint32(mapped[16:20], uint32(cap))
		binary.LittleEndian.PutUint32(mapped[20:24], date)
		size = 0
		capacity = uint32(cap)
		log.Printf("[rt] init %s type=%d cap=%d date=%d", fn, blkType, cap, date)
		return
	}
	// 已存在
	mapped, err = mm.MapRW(f, int(st.Size()))
	if err != nil { return }
	size = binary.LittleEndian.Uint32(mapped[12:16])
	capacity = binary.LittleEndian.Uint32(mapped[16:20])
	oldDate := binary.LittleEndian.Uint32(mapped[20:24])
	if oldDate != date {
		// 新交易日：清空 size 与数据区
		binary.LittleEndian.PutUint32(mapped[12:16], 0)
		binary.LittleEndian.PutUint32(mapped[20:24], date)
		size = 0
		log.Printf("[rt] reset date %s %d->%d", fn, oldDate, date)
	}
	return
}

// ensureCapacity 如果 size==capacity，则扩容为2倍并重新映射
func (w *Writer) ensureCapacity(f *os.File, mapped []byte, elemSize int, newCapacity uint32) ([]byte, error) {
	if err := mm.Unmap(mapped); err != nil { return nil, err }
	sz := rtHeaderLen + int(newCapacity)*elemSize
	if err := mm.EnsureSize(f, int64(sz)); err != nil { return nil, err }
	log.Printf("[rt] expand %s -> cap=%d", f.Name(), newCapacity)
	return mm.MapRW(f, sz)
}

// appendOne 将一条记录追加到 rt 文件（根据当前 size 写入，必要时扩容并更新 header）
func (w *Writer) appendOne(f *os.File, mapped []byte, elem []byte, elemSize int) ([]byte, error) {
	size := binary.LittleEndian.Uint32(mapped[12:16])
	capacity := binary.LittleEndian.Uint32(mapped[16:20])
	if size >= capacity {
		newCap := capacity * 2
		m2, err := w.ensureCapacity(f, mapped, elemSize, newCap)
		if err != nil { return mapped, err }
		mapped = m2
		binary.LittleEndian.PutUint32(mapped[16:20], newCap)
	}
	off := rtHeaderLen + int(size)*elemSize
	copy(mapped[off:off+elemSize], elem)
	binary.LittleEndian.PutUint32(mapped[12:16], size+1)
	return mapped, nil
}