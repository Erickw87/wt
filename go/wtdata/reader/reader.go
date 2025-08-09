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

func (r *Reader) lowerBoundTick(payload []byte, size uint32, etime uint64) int {
	// etime: yyyymmddhhmmsszzz -> 比较 action_date/action_time
	// 二分比较
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

// ---------------- Bars by count ----------------

// ReadKlineSlice 读取最后 count 根K线（对应 WtDataReader::readKlineSlice，简化：无主连/复权）
func (r *Reader) ReadKlineSlice(stdCode string, exchg string, code string, period int, count uint32, etime uint64) ([]types.WTSBarStruct, error) {
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
	// 追加 rt 当日部分
	rtSub := "min1"
	if period == types.KP_Minute5 {
		rtSub = "min5"
	}
	if period == types.KP_Minute1 || period == types.KP_Minute5 {
		rtPath := filepath.Join(r.rtDir, rtSub, exchg, fmt.Sprintf("%s.dmb", code))
		if st, err := os.Stat(rtPath); err == nil && st.Size() > 0 {
			_, size, payloadRt, err := rt.ReadKlineBlock(rtPath)
			if err == nil && size > 0 {
				cur := r.extractBarsTail(payloadRt, min(int(count)-len(bars), int(size)))
				bars = append(bars, cur...)
			}
		}
	}
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