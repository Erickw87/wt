package reader

// 主连（自定义规则）处理（对应 WtDataReader::cacheIntegratedBars 的简化实现）

import (
	"fmt"
	"os"
	"path/filepath"

	"wtdata/internal/codec"
	"wtdata/internal/rt"
	"wtdata/internal/types"
)

type HotSection struct {
	Code   string  // 分月合约原始代码
	SDate  uint32  // 左边界交易日（含）
	EDate  uint32  // 右边界交易日（含）
	Factor float64 // 该段复权因子（与 WT HotSection 一致含义）
}

// key: exchg.product.rule -> sections
var hotSections = map[string][]HotSection{}

// AddHotSections 注册主连分段
func AddHotSections(exchg, product, rule string, secs []HotSection) {
	key := fmt.Sprintf("%s.%s_%s", exchg, product, rule)
	hotSections[key] = secs
}

// loadHotCombinedBars 尝试直接读取合成的 hot 文件（his/{pname}/{exchg}/{exchg}.{product}_{rule}[Q/H].dsb）
func (r *Reader) loadHotCombinedBars(exchg, product, rule string, period int, exright byte) ([]byte, uint64, error) {
	pname := types.PERIOD_NAME[period]
	fn := filepath.Join(r.hisDir, pname, exchg, fmt.Sprintf("%s.%s_%s", exchg, product, rule))
	if exright == byte(types.SUFFIX_QFQ) {
		fn += string([]byte{types.SUFFIX_QFQ})
	} else if exright == byte(types.SUFFIX_HFQ) {
		fn += string([]byte{types.SUFFIX_HFQ})
	}
	fn += ".dsb"
	b, err := os.ReadFile(fn)
	if err != nil { return nil, 0, err }
	p, err := codec.ProcBlockData(b, true, false)
	if err != nil { return nil, 0, err }
	// 计算最后时间用于截断
	var last uint64
	cnt := len(p) / rt.SizeOfBarV2
	if cnt > 0 {
		// 读取最后一个 bar 的时间（分钟：bar.time；日：bar.date）
		bar := r.readBarAt(p, cnt-1)
		if period == types.KP_DAY {
			last = uint64(bar.Date)
		} else {
			last = bar.Time
		}
	}
	return p, last, nil
}

// integrateBarsWithSections 依据分段从历史分月文件抽取并拼接
func (r *Reader) integrateBarsWithSections(exchg, product, rule string, period int, exright byte, lastHot uint64) ([]types.WTSBarStruct, error) {
	pname := types.PERIOD_NAME[period]
	key := fmt.Sprintf("%s.%s_%s", exchg, product, rule)
	secs := hotSections[key]
	if len(secs) == 0 { return nil, fmt.Errorf("no hot sections for %s", key) }
	res := []types.WTSBarStruct{}
	// 基础因子：前复权以最后段因子为基准，后复权将作为 rt 段处理
	baseFactor := 1.0
	if exright == byte(types.SUFFIX_QFQ) {
		baseFactor = secs[len(secs)-1].Factor
	}
	for i := len(secs) - 1; i >= 0; i-- {
		sec := secs[i]
		fn := filepath.Join(r.hisDir, pname, exchg, fmt.Sprintf("%s.dsb", sec.Code))
		b, err := os.ReadFile(fn)
		if err != nil { continue }
		p, err := codec.ProcBlockData(b, true, false)
		if err != nil || len(p)==0 { continue }
		// 选取区间 [SDate, EDate]
		bars := make([]types.WTSBarStruct, len(p)/rt.SizeOfBarV2)
		for j:=0;j<len(bars);j++ { bars[j]=r.readBarAt(p,j) }
		// 将日期转换为时间比较
		// 过滤落在段内且不与 hot 文件重叠
		filtered := []types.WTSBarStruct{}
		for _, bar := range bars {
			in := false
			if period == types.KP_DAY { in = (bar.Date >= sec.SDate && bar.Date <= sec.EDate) } else {
				// bar.time 格式 ((date-19900000)*10000 + HHMM)
				in = (bar.Date >= sec.SDate && bar.Date <= sec.EDate)
			}
			if !in { continue }
			// 与 hot 段重叠剔除：当 bar 时间小于等于 lastHot 则跳过
			if period == types.KP_DAY {
				if lastHot != 0 && uint64(bar.Date) <= lastHot { continue }
			} else {
				if lastHot != 0 && bar.Time <= lastHot { continue }
			}
			// 复权（若需要）
			if exright == byte(types.SUFFIX_QFQ) {
				factor := sec.Factor / baseFactor
				bar.Open  *= factor; bar.High*=factor; bar.Low*=factor; bar.Close*=factor
				if (r.adjustFlg & 1) != 0 { bar.Vol/=factor }
				if (r.adjustFlg & 2) != 0 { bar.Money*=factor }
				if (r.adjustFlg & 4) != 0 { bar.Hold/=factor; bar.Add/=factor }
			}
			filtered = append(filtered, bar)
		}
		res = append(filtered, res...)
	}
	return res, nil
}