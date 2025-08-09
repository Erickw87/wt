package rdmreader

// 随机区间读取（对应 WtRdmDtReader）

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"wtdata/internal/codec"
	"wtdata/internal/rt"
	"wtdata/internal/types"
)

// Reader（对应 WtRdmDtReader）
type Reader struct {
	baseDir string
	// bars cache: key stdCode#period -> payload
	bars map[string][]byte
}

func (r *Reader) Init(base string) {
	if base != "" && base[len(base)-1] != '/' {
		base += "/"
	}
	r.baseDir = base
	r.bars = map[string][]byte{}
}

// ReadTickSliceByRange（对应 WtRdmDtReader::readTickSliceByRange），简化：按历史当日文件与 rt 拼接
func (r *Reader) ReadTickSliceByRange(stdCode, exchg, code string, stime, etime uint64) ([]types.WTSTickStruct, error) {
	// 先拼历史（仅目标 endTDate 当天），再拼 rt（若含当天）
	res := []types.WTSTickStruct{}
	// 历史
	endDate := uint32(etime / 1000000000)
	fn := filepath.Join(r.baseDir, "his", "ticks", exchg, fmt.Sprintf("%d", endDate), fmt.Sprintf("%s.dsb", code))
	if b, err := os.ReadFile(fn); err == nil {
		p, err := codec.ProcBlockData(b, false, true)
		if err == nil && len(p) >= 12 {
			pl := p[12:]
			cnt := len(pl) / rt.SizeOfTickV2
			if cnt > 0 {
				sIdx, eIdx := r.rangeIndexTicks(pl, stime, etime)
				if sIdx <= eIdx && eIdx < cnt {
					res = append(res, r.extractTicks(pl, sIdx, eIdx-sIdx+1)...)
				}
			}
		}
	}
	// 当日 rt（若 etime 所在交易日为今日）
	rtPath := filepath.Join(r.baseDir, "rt", "ticks", exchg, fmt.Sprintf("%s.dmb", code))
	if st, err := os.Stat(rtPath); err == nil && st.Size() > 0 {
		_, size, payload, err := rt.ReadTickBlock(rtPath)
		if err == nil && size > 0 {
			sIdx, eIdx := r.rangeIndexTicks(payload, stime, etime)
			if sIdx <= eIdx && eIdx < int(size) {
				res = append(res, r.extractTicks(payload, sIdx, eIdx-sIdx+1)...)
			}
		}
	}
	if len(res) == 0 {
		return nil, errors.New("no ticks in range")
	}
	log.Printf("[rdm] ticks %s.%s range %d-%d -> %d rows", exchg, code, stime, etime, len(res))
	return res, nil
}

func (r *Reader) rangeIndexTicks(payload []byte, stime, etime uint64) (int, int) {
	lDate := uint32(stime / 1000000000)
	lTime := uint32(stime % 1000000000)
	rDate := uint32(etime / 1000000000)
	rTime := uint32(etime % 1000000000)
	cnt := len(payload) / rt.SizeOfTickV2
	// lower_bound s
	s := 0
	e := cnt - 1
	for s <= e {
		m := (s + e) >> 1
		t := r.readTickAt(payload, m)
		if t.ActionDate < lDate || (t.ActionDate == lDate && t.ActionTime < lTime) {
			s = m + 1
		} else {
			e = m - 1
		}
	}
	sIdx := s
	// upper_bound e
	s = 0
	e = cnt - 1
	idx := -1
	for s <= e {
		m := (s + e) >> 1
		t := r.readTickAt(payload, m)
		if t.ActionDate < rDate || (t.ActionDate == rDate && t.ActionTime <= rTime) {
			idx = m
			s = m + 1
		} else {
			e = m - 1
		}
	}
	return sIdx, idx
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

// --------------------- Bars Range ---------------------

// ReadBarSliceByRange 读取区间 bars（分钟/日），拼接历史与当日 rt（分钟）
func (r *Reader) ReadBarSliceByRange(stdCode, exchg, code string, period int, stime, etime uint64) ([]types.WTSBarStruct, error) {
	pname := types.PERIOD_NAME[period]
	// 历史
	fn := filepath.Join(r.baseDir, "his", pname, exchg, fmt.Sprintf("%s.dsb", code))
	res := []types.WTSBarStruct{}
	if b, err := os.ReadFile(fn); err == nil {
		p, err := codec.ProcBlockData(b, true, false)
		if err == nil && len(p) > 0 {
			sIdx, eIdx := r.rangeIndexBars(p, period, stime, etime)
			if sIdx <= eIdx {
				res = append(res, r.extractBars(p, sIdx, eIdx-sIdx+1)...)
			}
		}
	}
	// 当日 rt 仅分钟
	if period == types.KP_Minute1 || period == types.KP_Minute5 {
		rtSub := "min1"
		if period == types.KP_Minute5 { rtSub = "min5" }
		rtPath := filepath.Join(r.baseDir, "rt", rtSub, exchg, fmt.Sprintf("%s.dmb", code))
		if st, err := os.Stat(rtPath); err == nil && st.Size() > 0 {
			_, size, payload, err := rt.ReadKlineBlock(rtPath)
			if err == nil && size > 0 {
				sIdx, eIdx := r.rangeIndexBars(payload, period, stime, etime)
				if sIdx <= eIdx && eIdx < int(size) {
					res = append(res, r.extractBars(payload, sIdx, eIdx-sIdx+1)...)
				}
			}
		}
	}
	if len(res) == 0 { return nil, errors.New("no bars in range") }
	log.Printf("[rdm] bars %s.%s %s range %d-%d -> %d rows", exchg, code, pname, stime, etime, len(res))
	return res, nil
}

func (r *Reader) rangeIndexBars(payload []byte, period int, stime, etime uint64) (int, int) {
	cnt := len(payload) / rt.SizeOfBarV2
	if cnt <= 0 { return 0, -1 }
	lDate := uint32(stime / 1000000000)
	lTime := uint32(stime % 1000000000)
	rDate := uint32(etime / 1000000000)
	rTime := uint32(etime % 1000000000)
	lHHMM := lTime / 100000
	rHHMM := rTime / 100000
	lKey := uint64((lDate-19900000))*10000 + uint64(lHHMM)
	rKey := uint64((rDate-19900000))*10000 + uint64(rHHMM)
	// lower_bound
	s, e := 0, cnt-1
	for s <= e {
		m := (s+e)>>1
		b := r.readBarAt(payload, m)
		var key uint64
		if period == types.KP_DAY {
			key = uint64(b.Date)
		} else {
			key = b.Time
		}
		if (period == types.KP_DAY && key < uint64(lDate)) || (period != types.KP_DAY && key < lKey) {
			s = m + 1
		} else {
			e = m - 1
		}
	}
	sIdx := s
	// upper_bound
	s, e = 0, cnt-1
	idx := -1
	for s <= e {
		m := (s+e)>>1
		b := r.readBarAt(payload, m)
		var key uint64
		if period == types.KP_DAY {
			key = uint64(b.Date)
		} else {
			key = b.Time
		}
		if (period == types.KP_DAY && key <= uint64(rDate)) || (period != types.KP_DAY && key <= rKey) {
			idx = m
			s = m + 1
		} else {
			e = m - 1
		}
	}
	return sIdx, idx
}

func (r *Reader) readBarAt(payload []byte, i int) types.WTSBarStruct {
	off := i * rt.SizeOfBarV2
	var k types.WTSBarStruct
	_ = binary.Read(bytes.NewReader(payload[off:off+rt.SizeOfBarV2]), binary.LittleEndian, &k)
	return k
}

func (r *Reader) extractBars(payload []byte, from int, n int) []types.WTSBarStruct {
	res := make([]types.WTSBarStruct, n)
	for i := 0; i < n; i++ { res[i] = r.readBarAt(payload, from+i) }
	return res
}

// --------------------- Order/Trans Range ---------------------

func (r *Reader) ReadOrdQueSliceByRange(stdCode, exchg, code string, stime, etime uint64) ([]types.WTSOrdQueStruct, error) {
	res := []types.WTSOrdQueStruct{}
	endDate := uint32(etime / 1000000000)
	fn := filepath.Join(r.baseDir, "his", "queue", exchg, fmt.Sprintf("%d", endDate), fmt.Sprintf("%s.dsb", code))
	if b, err := os.ReadFile(fn); err == nil {
		p, err := codec.ProcBlockData(b, false, true)
		if err == nil && len(p) >= 12 {
			pl := p[12:]
			cnt := len(pl) / rt.SizeOfOrdQue
			if cnt > 0 {
				sIdx, eIdx := r.rangeIndexOrdQue(pl, stime, etime)
				if sIdx <= eIdx && eIdx < cnt {
					res = append(res, r.extractOrdQue(pl, sIdx, eIdx-sIdx+1)...)
				}
			}
		}
	}
	rtPath := filepath.Join(r.baseDir, "rt", "queue", exchg, fmt.Sprintf("%s.dmb", code))
	if st, err := os.Stat(rtPath); err == nil && st.Size() > 0 {
		_, size, payload, err := rt.ReadOrdQueBlock(rtPath)
		if err == nil && size > 0 {
			sIdx, eIdx := r.rangeIndexOrdQue(payload, stime, etime)
			if sIdx <= eIdx && eIdx < int(size) {
				res = append(res, r.extractOrdQue(payload, sIdx, eIdx-sIdx+1)...)
			}
		}
	}
	if len(res) == 0 { return nil, errors.New("no ordque in range") }
	log.Printf("[rdm] ordque %s.%s range %d-%d -> %d rows", exchg, code, stime, etime, len(res))
	return res, nil
}

func (r *Reader) rangeIndexOrdQue(payload []byte, stime, etime uint64) (int, int) {
	lDate := uint32(stime / 1000000000)
	lTime := uint32(stime % 1000000000)
	rDate := uint32(etime / 1000000000)
	rTime := uint32(etime % 1000000000)
	cnt := len(payload) / rt.SizeOfOrdQue
	// lower_bound
	s, e := 0, cnt-1
	for s <= e {
		m := (s+e)>>1
		var v types.WTSOrdQueStruct
		_ = binary.Read(bytes.NewReader(payload[m*rt.SizeOfOrdQue:(m+1)*rt.SizeOfOrdQue]), binary.LittleEndian, &v)
		if v.ActionDate < lDate || (v.ActionDate == lDate && v.ActionTime < lTime) {
			s = m + 1
		} else {
			e = m - 1
		}
	}
	sIdx := s
	// upper_bound
	s, e = 0, cnt-1
	idx := -1
	for s <= e {
		m := (s+e)>>1
		var v types.WTSOrdQueStruct
		_ = binary.Read(bytes.NewReader(payload[m*rt.SizeOfOrdQue:(m+1)*rt.SizeOfOrdQue]), binary.LittleEndian, &v)
		if v.ActionDate < rDate || (v.ActionDate == rDate && v.ActionTime <= rTime) {
			idx = m
			s = m + 1
		} else {
			e = m - 1
		}
	}
	return sIdx, idx
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

func (r *Reader) ReadOrdDtlSliceByRange(stdCode, exchg, code string, stime, etime uint64) ([]types.WTSOrdDtlStruct, error) {
	res := []types.WTSOrdDtlStruct{}
	endDate := uint32(etime / 1000000000)
	fn := filepath.Join(r.baseDir, "his", "orders", exchg, fmt.Sprintf("%d", endDate), fmt.Sprintf("%s.dsb", code))
	if b, err := os.ReadFile(fn); err == nil {
		p, err := codec.ProcBlockData(b, false, true)
		if err == nil && len(p) >= 12 {
			pl := p[12:]
			cnt := len(pl) / rt.SizeOfOrdDtl
			if cnt > 0 {
				sIdx, eIdx := r.rangeIndexOrdDtl(pl, stime, etime)
				if sIdx <= eIdx && eIdx < cnt {
					res = append(res, r.extractOrdDtl(pl, sIdx, eIdx-sIdx+1)...)
				}
			}
		}
	}
	rtPath := filepath.Join(r.baseDir, "rt", "orders", exchg, fmt.Sprintf("%s.dmb", code))
	if st, err := os.Stat(rtPath); err == nil && st.Size() > 0 {
		_, size, payload, err := rt.ReadOrdDtlBlock(rtPath)
		if err == nil && size > 0 {
			sIdx, eIdx := r.rangeIndexOrdDtl(payload, stime, etime)
			if sIdx <= eIdx && eIdx < int(size) {
				res = append(res, r.extractOrdDtl(payload, sIdx, eIdx-sIdx+1)...)
			}
		}
	}
	if len(res) == 0 { return nil, errors.New("no orddtl in range") }
	log.Printf("[rdm] orddtl %s.%s range %d-%d -> %d rows", exchg, code, stime, etime, len(res))
	return res, nil
}

func (r *Reader) rangeIndexOrdDtl(payload []byte, stime, etime uint64) (int, int) {
	lDate := uint32(stime / 1000000000)
	lTime := uint32(stime % 1000000000)
	rDate := uint32(etime / 1000000000)
	rTime := uint32(etime % 1000000000)
	cnt := len(payload) / rt.SizeOfOrdDtl
	// lower_bound
	s, e := 0, cnt-1
	for s <= e {
		m := (s+e)>>1
		var v types.WTSOrdDtlStruct
		_ = binary.Read(bytes.NewReader(payload[m*rt.SizeOfOrdDtl:(m+1)*rt.SizeOfOrdDtl]), binary.LittleEndian, &v)
		if v.ActionDate < lDate || (v.ActionDate == lDate && v.ActionTime < lTime) {
			s = m + 1
		} else {
			e = m - 1
		}
	}
	sIdx := s
	// upper_bound
	s, e = 0, cnt-1
	idx := -1
	for s <= e {
		m := (s+e)>>1
		var v types.WTSOrdDtlStruct
		_ = binary.Read(bytes.NewReader(payload[m*rt.SizeOfOrdDtl:(m+1)*rt.SizeOfOrdDtl]), binary.LittleEndian, &v)
		if v.ActionDate < rDate || (v.ActionDate == rDate && v.ActionTime <= rTime) {
			idx = m
			s = m + 1
		} else {
			e = m - 1
		}
	}
	return sIdx, idx
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

func (r *Reader) ReadTransSliceByRange(stdCode, exchg, code string, stime, etime uint64) ([]types.WTSTransStruct, error) {
	res := []types.WTSTransStruct{}
	endDate := uint32(etime / 1000000000)
	fn := filepath.Join(r.baseDir, "his", "trans", exchg, fmt.Sprintf("%d", endDate), fmt.Sprintf("%s.dsb", code))
	if b, err := os.ReadFile(fn); err == nil {
		p, err := codec.ProcBlockData(b, false, true)
		if err == nil && len(p) >= 12 {
			pl := p[12:]
			cnt := len(pl) / rt.SizeOfTrans
			if cnt > 0 {
				sIdx, eIdx := r.rangeIndexTrans(pl, stime, etime)
				if sIdx <= eIdx && eIdx < cnt {
					res = append(res, r.extractTrans(pl, sIdx, eIdx-sIdx+1)...)
				}
			}
		}
	}
	rtPath := filepath.Join(r.baseDir, "rt", "trans", exchg, fmt.Sprintf("%s.dmb", code))
	if st, err := os.Stat(rtPath); err == nil && st.Size() > 0 {
		_, size, payload, err := rt.ReadTransBlock(rtPath)
		if err == nil && size > 0 {
			sIdx, eIdx := r.rangeIndexTrans(payload, stime, etime)
			if sIdx <= eIdx && eIdx < int(size) {
				res = append(res, r.extractTrans(payload, sIdx, eIdx-sIdx+1)...)
			}
		}
	}
	if len(res) == 0 { return nil, errors.New("no trans in range") }
	log.Printf("[rdm] trans %s.%s range %d-%d -> %d rows", exchg, code, stime, etime, len(res))
	return res, nil
}

func (r *Reader) rangeIndexTrans(payload []byte, stime, etime uint64) (int, int) {
	lDate := uint32(stime / 1000000000)
	lTime := uint32(stime % 1000000000)
	rDate := uint32(etime / 1000000000)
	rTime := uint32(etime % 1000000000)
	cnt := len(payload) / rt.SizeOfTrans
	// lower_bound
	s, e := 0, cnt-1
	for s <= e {
		m := (s+e)>>1
		var v types.WTSTransStruct
		_ = binary.Read(bytes.NewReader(payload[m*rt.SizeOfTrans:(m+1)*rt.SizeOfTrans]), binary.LittleEndian, &v)
		if v.ActionDate < lDate || (v.ActionDate == lDate && v.ActionTime < lTime) {
			s = m + 1
		} else {
			e = m - 1
		}
	}
	sIdx := s
	// upper_bound
	s, e = 0, cnt-1
	idx := -1
	for s <= e {
		m := (s+e)>>1
		var v types.WTSTransStruct
		_ = binary.Read(bytes.NewReader(payload[m*rt.SizeOfTrans:(m+1)*rt.SizeOfTrans]), binary.LittleEndian, &v)
		if v.ActionDate < rDate || (v.ActionDate == rDate && v.ActionTime <= rTime) {
			idx = m
			s = m + 1
		} else {
			e = m - 1
		}
	}
	return sIdx, idx
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