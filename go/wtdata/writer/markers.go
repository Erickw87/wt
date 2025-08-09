package writer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wtdata/internal/types"
)

// WriteMarker 更新交易会话标记（对应 marker.ini），key = sid, value = date
func (w *Writer) WriteMarker(sid string, date uint32) error {
	fn := filepath.Join(w.baseDir, "marker.ini")
	// 朴素 ini：一个 [markers] 段；此处简单覆盖写（后续可实现读取合并）
	content := fmt.Sprintf("[markers]\n%s=%d\n", sid, date)
	return os.WriteFile(fn, []byte(content), 0o644)
}

// DumpSnapshot 输出 tick 快照（对应 his/snapshot/{date}.csv）
func (w *Writer) DumpSnapshot(date uint32) error {
	if w.tcMap == nil { return nil }
	var sb strings.Builder
	sb.WriteString("date,exchg,code,open,high,low,close,settle,volume,turnover,openinterest,upperlimit,lowerlimit,preclose,presettle,preinterest\n")
	sz := w.tcSize
	for i := uint32(0); i < sz; i++ {
		off := tcHeaderLen + int(i)*tickCacheItemSize()
		var t types.WTSTickStruct
		_ = binary.Read(bytes.NewReader(w.tcMap[off+4:off+4+rtTickSize()]), binary.LittleEndian, &t)
		sb.WriteString(fmt.Sprintf("%d,%s,%s,%.8f,%.8f,%.8f,%.8f,%.8f,%.0f,%.0f,%.0f,%.8f,%.8f,%.8f,%.8f,%.8f\n",
			date, cString(t.Exchg[:]), cString(t.Code[:]), t.Open, t.High, t.Low, t.Price, t.SettlePrice,
			t.TotalVolume, t.TotalTurnover, t.OpenInterest, t.UpperLimit, t.LowerLimit, t.PreClose, t.PreSettle, t.PreInterest))
	}
	dir := filepath.Join(w.baseDir, "his", "snapshot")
	_ = os.MkdirAll(dir, 0o755)
	fn := filepath.Join(dir, fmt.Sprintf("%d.csv", date))
	return os.WriteFile(fn, []byte(sb.String()), 0o644)
}