package writer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"wtdata/internal/mm"
	"wtdata/internal/types"
)

const tcHeaderLen = 20 // RTBlockHeader: 12 + size(4) + capacity(4)
const tcStep = 200

func tickCacheItemSize() int { return 4 + rtTickSize() } // date + WTSTickStruct

// tick cache state
func (w *Writer) openTickCache() error {
	fn := filepath.Join(w.baseDir, "cache.dmb")
	f, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil { return err }
	w.tcFile = f
	st, _ := f.Stat()
	if st.Size() < tcHeaderLen {
		// init with 0 capacity
		if err := mm.EnsureSize(f, tcHeaderLen); err != nil { return err }
		m, err := mm.MapRW(f, tcHeaderLen)
		if err != nil { return err }
		w.tcMap = m
		copy(w.tcMap[0:8], []byte{'&','^','%','$','#','@','!',0})
		binary.LittleEndian.PutUint16(w.tcMap[8:10], uint16(types.BT_RT_Cache))
		binary.LittleEndian.PutUint16(w.tcMap[10:12], uint16(types.BLOCK_VERSION_RAW_V2))
		binary.LittleEndian.PutUint32(w.tcMap[12:16], 0) // size
		binary.LittleEndian.PutUint32(w.tcMap[16:20], 0) // capacity
		w.tcSize = 0
		w.tcCap = 0
		w.tcIdx = map[string]uint32{}
		return nil
	}
	m, err := mm.MapRW(f, int(st.Size()))
	if err != nil { return err }
	w.tcMap = m
	w.tcSize = binary.LittleEndian.Uint32(w.tcMap[12:16])
	w.tcCap  = binary.LittleEndian.Uint32(w.tcMap[16:20])
	w.tcIdx = map[string]uint32{}
	// rebuild index by scanning
	for i := uint32(0); i < w.tcSize; i++ {
		off := tcHeaderLen + int(i)*tickCacheItemSize()
		var t types.WTSTickStruct
		_ = binary.Read(bytes.NewReader(w.tcMap[off+4:off+4+rtTickSize()]), binary.LittleEndian, &t)
		key := fmt.Sprintf("%s.%s", cString(t.Exchg[:]), cString(t.Code[:]))
		w.tcIdx[key] = i
	}
	return nil
}

func (w *Writer) ensureTickCacheCap(capMin uint32) error {
	if w.tcCap >= capMin { return nil }
	newCap := w.tcCap
	if newCap == 0 { newCap = tcStep } else {
		for newCap < capMin { newCap += tcStep }
	}
	if err := mm.Unmap(w.tcMap); err != nil { return err }
	sz := tcHeaderLen + int(newCap)*tickCacheItemSize()
	if err := mm.EnsureSize(w.tcFile, int64(sz)); err != nil { return err }
	m, err := mm.MapRW(w.tcFile, sz)
	if err != nil { return err }
	w.tcMap = m
	binary.LittleEndian.PutUint32(w.tcMap[16:20], newCap)
	w.tcCap = newCap
	return nil
}

// UpdateTickCache 更新/追加一条缓存：
// - 基于 total_* 计算日内 volume/turnover/diff_interest
// - 郑商所同秒多笔：若同一秒内多条，则 action_time += 200ms（最大不超过该秒末）
// - 时间/量回退：丢弃并告警
func (w *Writer) UpdateTickCache(exchg, code string, date uint32, tick *types.WTSTickStruct) error {
	if w.tcFile == nil { if err := w.openTickCache(); err != nil { return err } }
	key := fmt.Sprintf("%s.%s", exchg, code)
	idx, ok := w.tcIdx[key]
	if !ok {
		// append new
		if err := w.ensureTickCacheCap(w.tcSize+1); err != nil { return err }
		idx = w.tcSize
		w.tcIdx[key] = idx
		w.tcSize++
		binary.LittleEndian.PutUint32(w.tcMap[12:16], w.tcSize)
	}
	off := tcHeaderLen + int(idx)*tickCacheItemSize()
	binary.LittleEndian.PutUint32(w.tcMap[off:off+4], date)
	// 读取上一条，计算差分
	var prev types.WTSTickStruct
	_ = binary.Read(bytes.NewReader(w.tcMap[off+4:off+4+rtTickSize()]), binary.LittleEndian, &prev)
	// 时间回退丢弃
	if prev.ActionDate > 0 {
		if tick.ActionDate < prev.ActionDate || (tick.ActionDate == prev.ActionDate && tick.ActionTime < prev.ActionTime) {
			log.Printf("[cache] drop time rollback %s.%s %d/%d -> %d/%d", exchg, code, prev.ActionDate, prev.ActionTime, tick.ActionDate, tick.ActionTime)
			return nil
		}
	}
	// 郑商所同秒多笔 +200ms 修正（示例规则：exchg=="CZCE" 且同 ActionDate 秒内 ActionTime==prev.ActionTime）
	if exchg == "CZCE" && prev.ActionDate == tick.ActionDate && tick.ActionTime == prev.ActionTime {
		// +200ms，ActionTime 是毫秒；最大限制在该秒末（+999）
		nt := tick.ActionTime + 200
		if nt/1000 == prev.ActionTime/1000 { tick.ActionTime = nt } else { tick.ActionTime = (tick.ActionTime/1000)*1000 + 999 }
	}
	// 差分字段：volume/turnover/diff_interest
	if tick.TotalVolume >= prev.TotalVolume {
		tick.Volume = tick.TotalVolume - prev.TotalVolume
	} else {
		log.Printf("[cache] vol rollback %s.%s total %.0f->%.0f", exchg, code, prev.TotalVolume, tick.TotalVolume)
		tick.Volume = 0
	}
	if tick.TotalTurnover >= prev.TotalTurnover {
		tick.TurnOver = tick.TotalTurnover - prev.TotalTurnover
	} else {
		log.Printf("[cache] amt rollback %s.%s total %.0f->%.0f", exchg, code, prev.TotalTurnover, tick.TotalTurnover)
		tick.TurnOver = 0
	}
	// OpenInterest 差分（允许负值）
	tick.DiffInterest = tick.OpenInterest - prev.OpenInterest
	// 写入
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, tick)
	copy(w.tcMap[off+4:off+4+rtTickSize()], buf.Bytes())
	return nil
}

// GetCurTick 从缓存读取当前 tick（对应 WtDataWriter::getCurTick）
func (w *Writer) GetCurTick(exchg, code string) (*types.WTSTickStruct, bool) {
	if w.tcFile == nil || w.tcMap == nil { return nil, false }
	key := fmt.Sprintf("%s.%s", exchg, code)
	idx, ok := w.tcIdx[key]
	if !ok { return nil, false }
	off := tcHeaderLen + int(idx)*tickCacheItemSize()
	var t types.WTSTickStruct
	_ = binary.Read(bytes.NewReader(w.tcMap[off+4:off+4+rtTickSize()]), binary.LittleEndian, &t)
	return &t, true
}

// helper: convert C-like fixed byte array to string (trim trailing zeros)
func cString(b []byte) string {
	i := 0
	for i < len(b) && b[i] != 0 { i++ }
	return string(b[:i])
}