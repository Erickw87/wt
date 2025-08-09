package reader

// Bars 复权处理（对应 WtDataReader 中对 _adjust_flag 的处理逻辑）

import (
	"wtdata/internal/adj"
	"wtdata/internal/types"
)

// applyPreAdjust 对历史 bars 做前复权（历史变小，以最后因子为基准）
func (r *Reader) applyPreAdjust(bars []types.WTSBarStruct, factor float64) {
	for i := range bars {
		bars[i].Open *= factor
		bars[i].High *= factor
		bars[i].Low *= factor
		bars[i].Close *= factor
		if (r.adjustFlg & 1) != 0 { bars[i].Vol /= factor }
		if (r.adjustFlg & 2) != 0 { bars[i].Money *= factor }
		if (r.adjustFlg & 4) != 0 {
			bars[i].Hold /= factor
			bars[i].Add  /= factor
		}
	}
}

// applyPostAdjust 对新增（rt）bars 做后复权（新数据变大，以最后因子为基准）
func (r *Reader) applyPostAdjust(bars []types.WTSBarStruct, factor float64) {
	for i := range bars {
		bars[i].Open *= factor
		bars[i].High *= factor
		bars[i].Low *= factor
		bars[i].Close *= factor
		// 后复权对 vol/money/hold/add 的处理与前复权同一标志规则
		if (r.adjustFlg & 1) != 0 { bars[i].Vol /= factor }
		if (r.adjustFlg & 2) != 0 { bars[i].Money *= factor }
		if (r.adjustFlg & 4) != 0 {
			bars[i].Hold /= factor
			bars[i].Add  /= factor
		}
	}
}

// adjustBarsPerDate 按日期应用复权因子（前/后复权均可使用，base 决定前后差异）
// base = 最后因子（前复权），或 1（后复权直接乘当前因子）
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
		if (flag & 4) != 0 {
			bars[i].Hold /= factor
			bars[i].Add  /= factor
		}
	}
}