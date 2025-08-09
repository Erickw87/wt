package writer

import (
	"bytes"
	"encoding/binary"
	"log"
	"os"
	"path/filepath"

	"wtdata/internal/codec"
	"wtdata/internal/rt"
	"wtdata/internal/types"
)

// aggregateDayFromRT 根据 rt/min1 的当日分钟条目聚合为 1 根日K
func (w *Writer) aggregateDayFromRT(exchg, code string, date uint32) (*types.WTSBarStruct, error) {
	rtPath := filepath.Join(w.baseDir, "rt", "min1", exchg, code+".dmb")
	st, err := os.Stat(rtPath)
	if err != nil || st.Size() < 24 { return nil, err }
	_, size, payload, err := rt.ReadKlineBlock(rtPath)
	if err != nil || size == 0 { return nil, err }
	cnt := len(payload) / rt.SizeOfBarV2
	var (
		first *types.WTSBarStruct
		last  *types.WTSBarStruct
		high  float64
		low   float64
		sumVol   float64
		sumMoney float64
		lastHold float64
		lastAdd  float64
	)
	for i := 0; i < cnt; i++ {
		var b types.WTSBarStruct
		_ = binary.Read(bytes.NewReader(payload[i*rt.SizeOfBarV2:(i+1)*rt.SizeOfBarV2]), binary.LittleEndian, &b)
		if b.Date != date { continue }
		if first == nil { bb := b; first = &bb; high = b.High; low = b.Low }
		bb := b; last = &bb
		if b.High > high { high = b.High }
		if b.Low  < low  { low  = b.Low }
		sumVol += b.Vol
		sumMoney += b.Money
		lastHold = b.Hold
		lastAdd  = b.Add
	}
	if first == nil || last == nil { return nil, nil }
	res := &types.WTSBarStruct{
		Date: date,
		Time: 0,
		Open: first.Open,
		High: high,
		Low:  low,
		Close: last.Close,
		Settle: last.Settle,
		Money: sumMoney,
		Vol:   sumVol,
		Hold:  lastHold,
		Add:   lastAdd,
	}
	return res, nil
}

// upsertDayToHis 将日K插入/替换到 his/day/{exchg}/{code}.dsb（CMP_V2），保持整个序列
func (w *Writer) upsertDayToHis(exchg, code string, bar *types.WTSBarStruct) error {
	dir := filepath.Join(w.baseDir, "his", "day", exchg)
	_ = os.MkdirAll(dir, 0o755)
	fn := filepath.Join(dir, code+".dsb")
	var days []types.WTSBarStruct
	if b, err := os.ReadFile(fn); err == nil && len(b) > 0 {
		p, err := codec.ProcBlockData(b, true, false)
		if err == nil && len(p) > 0 {
			cnt := len(p) / rt.SizeOfBarV2
			days = make([]types.WTSBarStruct, cnt)
			for i:=0;i<cnt;i++ {
				_ = binary.Read(bytes.NewReader(p[i*rt.SizeOfBarV2:(i+1)*rt.SizeOfBarV2]), binary.LittleEndian, &days[i])
			}
		}
	}
	// 替换同日期或追加
	replaced := false
	for i := range days {
		if days[i].Date == bar.Date { days[i] = *bar; replaced = true; break }
	}
	if !replaced { days = append(days, *bar) }
	// 重新序列化并压缩
	buf := &bytes.Buffer{}
	for i := range days { _ = binary.Write(buf, binary.LittleEndian, &days[i]) }
	cmp := codec.CompressZstd(buf.Bytes())
	head := make([]byte, 20)
	copy(head[0:8], []byte{'&','^','%','$','#','@','!',0})
	binary.LittleEndian.PutUint16(head[8:10], uint16(types.BT_HIS_Day))
	binary.LittleEndian.PutUint16(head[10:12], uint16(types.BLOCK_VERSION_CMP_V2))
	binary.LittleEndian.PutUint64(head[12:20], uint64(len(cmp)))
	log.Printf("[close] upsert day %s -> %s bars=%d", code, fn, len(days))
	return os.WriteFile(fn, append(head, cmp...), 0o644)
}