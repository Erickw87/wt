package rdmreader

import (
	"wtdata/internal/adj"
	"wtdata/internal/types"
)

// adjustBarsPerDate 按日期应用复权因子（base = 最后因子（前复权），或 1（后复权直接乘当前因子)）
func adjustBarsPerDate(bars []types.WTSBarStruct, amap adj.Map, key string, base float64, flag uint32) {
	if len(bars)==0 { return }
	for i := range bars {
		f := adj.GetFactorByDate(amap, key, bars[i].Date)
		factor := f / base
		bars[i].Open  *= factor
		bars[i].High  *= factor
		bars[i].Low   *= factor
		bars[i].Close *= factor
		if (flag & 1) != 0 { bars[i].Vol /= factor }
		if (flag & 2) != 0 { bars[i].Money *= factor }
		if (flag & 4) != 0 { bars[i].Hold /= factor; bars[i].Add /= factor }
	}
}