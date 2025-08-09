package reader

// 在线数据读取（对应 WtDataReader）

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"wtdata/internal/adj"
	"wtdata/internal/codec"
	"wtdata/internal/rt"
	"wtdata/internal/types"
)

// Sink/Loader/HotMgr 等接口与外部系统关联，Go 版此处留空或由上层注入

// Reader（对应 WtDataReader）
type Reader struct {
	rtDir     string
	hisDir    string
	adjustFlg uint32 // 成交量/成交额/持仓复权位（对应 _adjust_flag）

	// 历史缓存（类似 _his_*_map）
	hisTick map[string][]byte // key: stdCode-date
	hisODtl map[string][]byte
	hisOQue map[string][]byte
	hisTran map[string][]byte

	// bars 缓存（类似 _bars_cache）
	bars map[string][]byte // key: stdCode#period -> payload (WTSBarStruct[])

	// 除权因子
	adjMap adj.Map
}

func (r *Reader) Init(base string, hisPath string, adjustFlag uint32) {
	if base != "" && base[len(base)-1] != '/' {
		base += "/"
	}
	if hisPath == "" {
		hisPath = base + "his/"
	} else if hisPath[len(hisPath)-1] != '/' {
		hisPath += "/"
	}
	r.rtDir = base + "rt/"
	r.hisDir = hisPath
	r.adjustFlg = adjustFlag
	r.hisTick = map[string][]byte{}
	r.hisODtl = map[string][]byte{}
	r.hisOQue = map[string][]byte{}
	r.hisTran = map[string][]byte{}
	r.bars = map[string][]byte{}
	r.adjMap = adj.Map{}
}

// LoadAdjFactorsFromFile（对应 WtDataReader::loadStkAdjFactorsFromFile）
func (r *Reader) LoadAdjFactorsFromFile(path string) error {
	m, err := adj.LoadFromFile(path)
	if err != nil { return err }
	r.adjMap = m
	return nil
}

// ---------------- Tick/Order/Trans by count ----------------

// ReadTickSlice 读取最后 count 条tick（对应 WtDataReader::readTickSlice，简化：不含自定义主连）
func (r *Reader) ReadTickSlice(stdCode string, exchg string, code string, count uint32, etime uint64) ([]types.WTSTickStruct, error) {
	// 优先从 rt 读取，再不足从历史最近日期补齐
	rtPath := filepath.Join(r.rtDir, "ticks", exchg, fmt.Sprintf("%s.dmb", code))
	var res []types.WTSTickStruct
	if st, err := os.Stat(rtPath); err == nil && st.Size() > 0 {
		date, size, payload, err := rt.ReadTickBlock(rtPath)
		_ = date
		if err == nil && size > 0 {
			// lower_bound 到 etime
			iEnd := r.lowerBoundTick(payload, size, etime)
			if iEnd >= 0 {
				s := int(count)
				if s > iEnd+1 {
					s = iEnd + 1
				}
				res = r.extractTicks(payload, iEnd+1-s, s)
				if uint32(len(res)) == count {
					return res, nil
				}
			}
		}
	}
	// 历史补齐：需要最近交易日 .dsb；此处简化：尝试按当日目录向前找最多 30 天
	missing := int(count) - len(res)
	if missing <= 0 {
		return res, nil
	}
	nowDate := r.currentTradingDate()
	for d := 0; d < 30 && missing > 0; d++ {
		day := nowDate - uint32(d)
		fn := filepath.Join(r.hisDir, "ticks", exchg, fmt.Sprintf("%d", day), fmt.Sprintf("%s.dsb", code))
		b, err := os.ReadFile(fn)
		if err != nil {
			continue
		}
		p, err := codec.ProcBlockData(b, false, true)
		if err != nil || len(p) < 12 {
			continue
		}
		// 去头
		payload := p[12:]
		// 末尾截取 missing 条
		cnt := (len(payload)) / rt.SizeOfTickV2
		if cnt <= 0 {
			continue
		}
		s := missing
		if s > cnt {
			s = cnt
		}
		chunk := r.extractTicks(payload, cnt-s, s)
		res = append(chunk, res...)
		missing = int(count) - len(res)
	}
	if len(res) == 0 {
		return nil, errors.New("no ticks available")
	}
	return res, nil
}

// ReadOrdQueSlice 读取最后 count 条委托队列（对应 WtDataReader::readOrdQueSlice）
func (r *Reader) ReadOrdQueSlice(stdCode, exchg, code string, count uint32, etime uint64) ([]types.WTSOrdQueStruct, error) {
	rtPath := filepath.Join(r.rtDir, "queue", exchg, fmt.Sprintf("%s.dmb", code))
	var res []types.WTSOrdQueStruct
	if st, err := os.Stat(rtPath); err == nil && st.Size() > 0 {
		_, size, payload, err := rt.ReadOrdQueBlock(rtPath)
		if err == nil && size > 0 {
			iEnd := r.lowerBoundOrdQue(payload, size, etime)
			if iEnd >= 0 {
				s := int(count)
				if s > iEnd+1 { s = iEnd+1 }
				res = r.extractOrdQue(payload, iEnd+1-s, s)
				if uint32(len(res)) == count { return res, nil }
			}
		}
	}
	missing := int(count) - len(res)
	if missing <= 0 { return res, nil }
	nowDate := r.currentTradingDate()
	for d := 0; d < 30 && missing > 0; d++ {
		day := nowDate - uint32(d)
		fn := filepath.Join(r.hisDir, "queue", exchg, fmt.Sprintf("%d", day), fmt.Sprintf("%s.dsb", code))
		b, err := os.ReadFile(fn)
		if err != nil { continue }
		p, err := codec.ProcBlockData(b, false, true)
		if err != nil || len(p) < 12 { continue }
		payload := p[12:]
		cnt := len(payload) / rt.SizeOfOrdQue
		if cnt <= 0 { continue }
		s := missing; if s > cnt { s = cnt }
		chunk := r.extractOrdQue(payload, cnt-s, s)
		res = append(chunk, res...)
		missing = int(count) - len(res)
	}
	if len(res) == 0 { return nil, errors.New("no ordque available") }
	return res, nil
}

// ReadOrdDtlSlice（对应 WtDataReader::readOrdDtlSlice）
func (r *Reader) ReadOrdDtlSlice(stdCode, exchg, code string, count uint32, etime uint64) ([]types.WTSOrdDtlStruct, error) {
	rtPath := filepath.Join(r.rtDir, "orders", exchg, fmt.Sprintf("%s.dmb", code))
	var res []types.WTSOrdDtlStruct
	if st, err := os.Stat(rtPath); err == nil && st.Size() > 0 {
		_, size, payload, err := rt.ReadOrdDtlBlock(rtPath)
		if err == nil && size > 0 {
			iEnd := r.lowerBoundOrdDtl(payload, size, etime)
			if iEnd >= 0 {
				s := int(count)
				if s > iEnd+1 { s = iEnd+1 }
				res = r.extractOrdDtl(payload, iEnd+1-s, s)
				if uint32(len(res)) == count { return res, nil }
			}
		}
	}
	missing := int(count) - len(res)
	if missing <= 0 { return res, nil }
	nowDate := r.currentTradingDate()
	for d := 0; d < 30 && missing > 0; d++ {
		day := nowDate - uint32(d)
		fn := filepath.Join(r.hisDir, "orders", exchg, fmt.Sprintf("%d", day), fmt.Sprintf("%s.dsb", code))
		b, err := os.ReadFile(fn)
		if err != nil { continue }
		p, err := codec.ProcBlockData(b, false, true)
		if err != nil || len(p) < 12 { continue }
		payload := p[12:]
		cnt := len(payload) / rt.SizeOfOrdDtl
		if cnt <= 0 { continue }
		s := missing; if s > cnt { s = cnt }
		chunk := r.extractOrdDtl(payload, cnt-s, s)
		res = append(chunk, res...)
		missing = int(count) - len(res)
	}
	if len(res) == 0 { return nil, errors.New("no orddtl available") }
	return res, nil
}

// ReadTransSlice（对应 WtDataReader::readTransSlice）
func (r *Reader) ReadTransSlice(stdCode, exchg, code string, count uint32, etime uint64) ([]types.WTSTransStruct, error) {
	rtPath := filepath.Join(r.rtDir, "trans", exchg, fmt.Sprintf("%s.dmb", code))
	var res []types.WTSTransStruct
	if st, err := os.Stat(rtPath); err == nil && st.Size() > 0 {
		_, size, payload, err := rt.ReadTransBlock(rtPath)
		if err == nil && size > 0 {
			iEnd := r.lowerBoundTrans(payload, size, etime)
			if iEnd >= 0 {
				s := int(count)
				if s > iEnd+1 { s = iEnd+1 }
				res = r.extractTrans(payload, iEnd+1-s, s)
				if uint32(len(res)) == count { return res, nil }
			}
		}
	}
	missing := int(count) - len(res)
	if missing <= 0 { return res, nil }
	nowDate := r.currentTradingDate()
	for d := 0; d < 30 && missing > 0; d++ {
		day := nowDate - uint32(d)
		fn := filepath.Join(r.hisDir, "trans", exchg, fmt.Sprintf("%d", day), fmt.Sprintf("%s.dsb", code))
		b, err := os.ReadFile(fn)
		if err != nil { continue }
		p, err := codec.ProcBlockData(b, false, true)
		if err != nil || len(p) < 12 { continue }
		payload := p[12:]
		cnt := len(payload) / rt.SizeOfTrans
		if cnt <= 0 { continue }
		s := missing; if s > cnt { s = cnt }
		chunk := r.extractTrans(payload, cnt-s, s)
		res = append(chunk, res...)
		missing = int(count) - len(res)
	}
	if len(res) == 0 { return nil, errors.New("no trans available") }
	return res, nil
}

// --- lower_bound helpers ---

func (r *Reader) lowerBoundTick(payload []byte, size uint32, etime uint64) int {
	// etime: yyyymmddhhmmsszzz -> 比较 action_date/action_time
	l, h := 0, int(size)-1
	keyDate := uint32(etime / 1000000000)
	keyTime := uint32(etime % 1000000000)
	var idx int
	for l <= h {
		m := (l + h) >> 1
		a := r.readTickAt(payload, m)
		if a.ActionDate < keyDate || (a.ActionDate == keyDate && a.ActionTime < keyTime) {
			idx = m
			l = m + 1
		} else {
			h = m - 1
		}
	}
	return idx
}

func (r *Reader) lowerBoundOrdQue(payload []byte, size uint32, etime uint64) int {
	l, h := 0, int(size)-1
	keyDate := uint32(etime / 1000000000)
	keyTime := uint32(etime % 1000000000)
	var idx int
	for l <= h {
		m := (l + h) >> 1
		var q types.WTSOrdQueStruct
		_ = binary.Read(bytes.NewReader(payload[m*rt.SizeOfOrdQue:(m+1)*rt.SizeOfOrdQue]), binary.LittleEndian, &q)
		if q.ActionDate < keyDate || (q.ActionDate == keyDate && q.ActionTime < keyTime) {
			idx = m; l = m + 1
		} else { h = m - 1 }
	}
	return idx
}

func (r *Reader) lowerBoundOrdDtl(payload []byte, size uint32, etime uint64) int {
	l, h := 0, int(size)-1
	keyDate := uint32(etime / 1000000000)
	keyTime := uint32(etime % 1000000000)
	var idx int
	for l <= h {
		m := (l + h) >> 1
		var q types.WTSOrdDtlStruct
		_ = binary.Read(bytes.NewReader(payload[m*rt.SizeOfOrdDtl:(m+1)*rt.SizeOfOrdDtl]), binary.LittleEndian, &q)
		if q.ActionDate < keyDate || (q.ActionDate == keyDate && q.ActionTime < keyTime) {
			idx = m; l = m + 1
		} else { h = m - 1 }
	}
	return idx
}

func (r *Reader) lowerBoundTrans(payload []byte, size uint32, etime uint64) int {
	l, h := 0, int(size)-1
	keyDate := uint32(etime / 1000000000)
	keyTime := uint32(etime % 1000000000)
	var idx int
	for l <= h {
		m := (l + h) >> 1
		var q types.WTSTransStruct
		_ = binary.Read(bytes.NewReader(payload[m*rt.SizeOfTrans:(m+1)*rt.SizeOfTrans]), binary.LittleEndian, &q)
		if q.ActionDate < keyDate || (q.ActionDate == keyDate && q.ActionTime < keyTime) {
			idx = m; l = m + 1
		} else { h = m - 1 }
	}
	return idx
}

// --- element readers ---

func (r *Reader) readTickAt(payload []byte, i int) types.WTSTickStruct {
	off := i * rt.SizeOfTickV2
	var t types.WTSTickStruct
	_ = binary.Read(bytes.NewReader(payload[off:off+rt.SizeOfTickV2]), binary.LittleEndian, &t)
	return t
}

func (r *Reader) extractTicks(payload []byte, from int, n int) []types.WTSTickStruct {
	res := make([]types.WTSTickStruct, n)
	for i := 0; i < n; i++ {
		res[i] = r.readTickAt(payload, from+i)
	}
	return res
}

func (r *Reader) extractOrdQue(payload []byte, from int, n int) []types.WTSOrdQueStruct {
	res := make([]types.WTSOrdQueStruct, n)
	for i := 0; i < n; i++ {
		var v types.WTSOrdQueStruct
		_ = binary.Read(bytes.NewReader(payload[(from+i)*rt.SizeOfOrdQue:(from+i+1)*rt.SizeOfOrdQue]), binary.LittleEndian, &v)
		res[i] = v
	}
	return res
}

func (r *Reader) extractOrdDtl(payload []byte, from int, n int) []types.WTSOrdDtlStruct {
	res := make([]types.WTSOrdDtlStruct, n)
	for i := 0; i < n; i++ {
		var v types.WTSOrdDtlStruct
		_ = binary.Read(bytes.NewReader(payload[(from+i)*rt.SizeOfOrdDtl:(from+i+1)*rt.SizeOfOrdDtl]), binary.LittleEndian, &v)
		res[i] = v
	}
	return res
}

func (r *Reader) extractTrans(payload []byte, from int, n int) []types.WTSTransStruct {
	res := make([]types.WTSTransStruct, n)
	for i := 0; i < n; i++ {
		var v types.WTSTransStruct
		_ = binary.Read(bytes.NewReader(payload[(from+i)*rt.SizeOfTrans:(from+i+1)*rt.SizeOfTrans]), binary.LittleEndian, &v)
		res[i] = v
	}
	return res
}

// ---------------- Bars by count ----------------

// ReadKlineSlice 读取最后 count 根K线（对应 WtDataReader::readKlineSlice，支持 QFQ/HFQ 简化复权 + 主连）
func (r *Reader) ReadKlineSlice(stdCode string, exchg string, code string, period int, count uint32, etime uint64) ([]types.WTSBarStruct, error) {
	// 主连处理：stdCode 形如 EXCHG.PRODUCT_rule[-|+]?
	if ex, prod, rule, xr, ok := parseHotStd(stdCode); ok {
		// 优先读取已合成 hot 文件
		if payload, lastHot, err := r.loadHotCombinedBars(ex, prod, rule, period, xr); err == nil && len(payload) > 0 {
			bars := r.extractBarsTail(payload, int(count))
			// 追加 rt：取当前段代码
			if period == types.KP_Minute1 || period == types.KP_Minute5 {
				curCode := currentHotCode(ex, prod, rule, r.currentTradingDate())
				rtSub := "min1"; if period == types.KP_Minute5 { rtSub = "min5" }
				rtPath := filepath.Join(r.rtDir, rtSub, ex, fmt.Sprintf("%s.dmb", curCode))
				if st, e := os.Stat(rtPath); e == nil && st.Size() > 0 {
					_, size, payloadRt, e2 := rt.ReadKlineBlock(rtPath)
					if e2 == nil && size > 0 {
						// 截取 > lastHot 的条目
						app := []types.WTSBarStruct{}
						cnt := len(payloadRt) / rt.SizeOfBarV2
						for i:=0;i<cnt;i++ { b:=r.readBarAt(payloadRt,i); if b.Time>lastHot { app = append(app,b) } }
						// 复权：QFQ 按 currentFactor/baseFactor；HFQ 按 currentFactor
						if xr == byte(types.SUFFIX_QFQ) || xr == byte(types.SUFFIX_HFQ) {
							cf := currentHotFactor(ex, prod, rule, r.currentTradingDate())
							if xr == byte(types.SUFFIX_QFQ) {
								bf := latestHotFactor(ex, prod, rule)
								applyHotFactorToBars(app, cf/bf, r.adjustFlg)
							} else {
								applyHotFactorToBars(app, cf, r.adjustFlg)
							}
						}
						bars = append(bars, app...)
						if int(count) < len(bars) { bars = bars[len(bars)-int(count):] }
					}
				}
			}
			if len(bars) == 0 { return nil, errors.New("no bars available") }
			return bars, nil
		}
		// 无合成文件，则按分段拼接
		var lastHot uint64
		if period == types.KP_Minute1 || period == types.KP_Minute5 {
			// 若存在合成文件可得 lastHot；此处无合成，lastHot=0
		}
		bars, err := r.integrateBarsWithSections(ex, prod, rule, period, xr, lastHot)
		if err == nil {
			// 追加 rt
			var app []types.WTSBarStruct
			if period == types.KP_Minute1 || period == types.KP_Minute5 {
				curCode := currentHotCode(ex, prod, rule, r.currentTradingDate())
				rtSub := "min1"; if period == types.KP_Minute5 { rtSub = "min5" }
				rtPath := filepath.Join(r.rtDir, rtSub, ex, fmt.Sprintf("%s.dmb", curCode))
				if st, e := os.Stat(rtPath); e == nil && st.Size() > 0 {
					_, size, payloadRt, e2 := rt.ReadKlineBlock(rtPath)
					if e2 == nil && size > 0 {
						cnt := len(payloadRt) / rt.SizeOfBarV2
						for i:=0;i<cnt;i++ { app = append(app, r.readBarAt(payloadRt,i)) }
					}
				}
				// 复权 rt 段
				if xr == byte(types.SUFFIX_QFQ) || xr == byte(types.SUFFIX_HFQ) {
					cf := currentHotFactor(ex, prod, rule, r.currentTradingDate())
					if xr == byte(types.SUFFIX_QFQ) { bf := latestHotFactor(ex, prod, rule); applyHotFactorToBars(app, cf/bf, r.adjustFlg) } else { applyHotFactorToBars(app, cf, r.adjustFlg) }
				}
			}
			bars = append(bars, app...)
			if int(count) < len(bars) { bars = bars[len(bars)-int(count):] }
			if len(bars) == 0 { return nil, errors.New("no bars available") }
			return bars, nil
		}
	}

	// 非主连：走普通逻辑
	key := fmt.Sprintf("%s#%d", stdCode, period)
	payload := r.bars[key]
	if payload == nil {
		b, err := r.loadHisBars(exchg, code, period)
		if err != nil {
			return nil, err
		}
		payload = b
		r.bars[key] = payload
	}
	bars := r.extractBarsTail(payload, int(count))
	// 复权检测（标准代码尾部 '-' 前复权，'+' 后复权）
	var exright byte
	if len(stdCode) > 0 {
		exright = stdCode[len(stdCode)-1]
	}
	// 追加 rt 当日部分
	rtSub := "min1"
	if period == types.KP_Minute5 {
		rtSub = "min5"
	}
	var rtBars []types.WTSBarStruct
	if period == types.KP_Minute1 || period == types.KP_Minute5 {
		rtPath := filepath.Join(r.rtDir, rtSub, exchg, fmt.Sprintf("%s.dmb", code))
		if st, err := os.Stat(rtPath); err == nil && st.Size() > 0 {
			_, size, payloadRt, err := rt.ReadKlineBlock(rtPath)
			if err == nil && size > 0 {
				cur := r.extractBarsTail(payloadRt, min(int(count)-len(bars), int(size)))
				rtBars = append(rtBars, cur...)
			}
		}
	}
	// 复权应用
	if exright == byte(types.SUFFIX_QFQ) || exright == byte(types.SUFFIX_HFQ) {
		// 构造因子键（缺省使用 STK PID）
		fkey := fmt.Sprintf("%s.STK.%s", exchg, code)
		base := adj.GetLastFactor(r.adjMap, fkey)
		if exright == byte(types.SUFFIX_QFQ) {
			// 历史部分按日期各自折算为 factor/date / base
			adjustBarsPerDate(bars, r.adjMap, fkey, base, r.adjustFlg)
		} else if exright == byte(types.SUFFIX_HFQ) {
			// 后复权：仅对 rt 追加部分缩放
			adjustBarsPerDate(rtBars, r.adjMap, fkey, 1.0, r.adjustFlg) // base=1 表示直接按当前因子缩放
		}
	}
	bars = append(bars, rtBars...)
	if len(bars) == 0 {
		return nil, errors.New("no bars available")
	}
	return bars, nil
}

func (r *Reader) loadHisBars(exchg, code string, period int) ([]byte, error) {
	pname := types.PERIOD_NAME[period]
	fn := filepath.Join(r.hisDir, pname, exchg, fmt.Sprintf("%s.dsb", code))
	b, err := os.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	p, err := codec.ProcBlockData(b, true, false)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *Reader) extractBarsTail(payload []byte, n int) []types.WTSBarStruct {
	if n <= 0 {
		return nil
	}
	cnt := len(payload) / rt.SizeOfBarV2
	if cnt <= 0 {
		return nil
	}
	if n > cnt {
		n = cnt
	}
	res := make([]types.WTSBarStruct, n)
	start := cnt - n
	for i := 0; i < n; i++ {
		res[i] = r.readBarAt(payload, start+i)
	}
	return res
}

func (r *Reader) readBarAt(payload []byte, i int) types.WTSBarStruct {
	off := i * rt.SizeOfBarV2
	var k types.WTSBarStruct
	_ = binary.Read(bytes.NewReader(payload[off:off+rt.SizeOfBarV2]), binary.LittleEndian, &k)
	return k
}

// ---------------- helpers ----------------

func (r *Reader) currentTradingDate() uint32 { return 20250101 } // 上层应注入，简化为固定值

func min(a, b int) int { if a < b { return a }; return b }

// lower_bound 一般形式（本文件中针对 tick 单独实现）
func lowerBound[T any](n int, less func(i int) bool) int {
	idx := sort.Search(n, less)
	return idx
}