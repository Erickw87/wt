package reader

// 主连（自定义规则）处理（对应 WtDataReader::cacheIntegratedBars 的简化实现）

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// parseHotStd 解析 stdCode 中的 product_rule（形如 EXCHG.PRODUCT_rule[-|+]?）
func parseHotStd(stdCode string) (exchg, product, rule string, exright byte, ok bool) {
	if stdCode == "" { return "","","",0,false }
	// 取末尾复权标识
	last := stdCode[len(stdCode)-1]
	if last == byte(types.SUFFIX_QFQ) || last == byte(types.SUFFIX_HFQ) {
		exright = last
		stdCode = stdCode[:len(stdCode)-1]
	}
	parts := strings.Split(stdCode, ".")
	if len(parts) < 2 { return "","","",exright,false }
	exchg = parts[0]
	p := parts[1]
	if idx := strings.IndexByte(p, '_'); idx > 0 && idx < len(p)-1 {
		product = p[:idx]
		rule = p[idx+1:]
		ok = true
	}
	return
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
		// 过滤落在段内且不与 hot 文件重叠
		filtered := []types.WTSBarStruct{}
		for _, bar := range bars {
			in := false
			if period == types.KP_DAY { in = (bar.Date >= sec.SDate && bar.Date <= sec.EDate) } else {
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
				applyHotFactorToBar(&bar, factor,  r.adjustFlg)
			}
			filtered = append(filtered, bar)
		}
		res = append(filtered, res...)
	}
	return res, nil
}

func latestHotFactor(exchg, product, rule string) float64 {
	key := fmt.Sprintf("%s.%s_%s", exchg, product, rule)
	secs := hotSections[key]
	if len(secs) == 0 { return 1 }
	return secs[len(secs)-1].Factor
}

func currentHotFactor(exchg, product, rule string, date uint32) float64 {
	key := fmt.Sprintf("%s.%s_%s", exchg, product, rule)
	secs := hotSections[key]
	if len(secs) == 0 { return 1 }
	for i := len(secs)-1; i>=0; i-- {
		if date >= secs[i].SDate && date <= secs[i].EDate { return secs[i].Factor }
	}
	return secs[len(secs)-1].Factor
}

func currentHotCode(exchg, product, rule string, date uint32) string {
	key := fmt.Sprintf("%s.%s_%s", exchg, product, rule)
	secs := hotSections[key]
	if len(secs) == 0 { return "" }
	for i := len(secs)-1; i>=0; i-- {
		if date >= secs[i].SDate && date <= secs[i].EDate { return secs[i].Code }
	}
	return secs[len(secs)-1].Code
}

func applyHotFactorToBar(bar *types.WTSBarStruct, factor float64, flag uint32) {
	bar.Open  *= factor
	bar.High  *= factor
	bar.Low   *= factor
	bar.Close *= factor
	if (flag & 1) != 0 { bar.Vol   /= factor }
	if (flag & 2) != 0 { bar.Money *= factor }
	if (flag & 4) != 0 { bar.Hold  /= factor; bar.Add /= factor }
}

func applyHotFactorToBars(bars []types.WTSBarStruct, factor float64, flag uint32) {
	for i := range bars { applyHotFactorToBar(&bars[i], factor, flag) }
}

// scaleBarsByHotSections 对拼接后的 bars 按所在段应用因子（base 为 QFQ 的最后段因子或 HFQ 的 1）
func scaleBarsByHotSections(bars []types.WTSBarStruct, exchg, product, rule string, base float64, flag uint32) {
	key := fmt.Sprintf("%s.%s_%s", exchg, product, rule)
	secs := hotSections[key]
	if len(secs) == 0 { return }
	for i := range bars {
		f := 1.0
		for j := len(secs)-1; j>=0; j-- {
			if bars[i].Date >= secs[j].SDate && bars[i].Date <= secs[j].EDate { f = secs[j].Factor; break }
		}
		applyHotFactorToBar(&bars[i], f/base, flag)
	}
}

// SetCustomRule 外部注入主连分段（简化版），同 AddHotSections
func (r *Reader) SetCustomRule(exchg, product, rule string, secs []HotSection) {
	AddHotSections(exchg, product, rule, secs)
}