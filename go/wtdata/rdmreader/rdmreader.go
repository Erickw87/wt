package rdmreader

// 随机区间读取（对应 WtRdmDtReader）

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
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