package reader

// Bars 复权处理（对应 WtDataReader 中对 _adjust_flag 的处理逻辑）

import (
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