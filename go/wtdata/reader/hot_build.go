package reader

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"wtdata/internal/codec"
	"wtdata/internal/types"
)

// BuildHotCombined 生成主连合成历史文件 his/{pname}/{exchg}/{exchg}.{product}_{rule}[Q/H].dsb
// exright: '-' 前复权，'+' 后复权，0 表示不复权
func (r *Reader) BuildHotCombined(exchg, product, rule string, period int, exright byte) error {
	pname := types.PERIOD_NAME[period]
	// 先按分段拼接
	bars, err := r.integrateBarsWithSections(exchg, product, rule, period, 0, 0)
	if err != nil {
		return err
	}
	if len(bars) == 0 {
		return fmt.Errorf("no bars for %s.%s_%s", exchg, product, rule)
	}
	// 复权
	if exright == byte(types.SUFFIX_QFQ) || exright == byte(types.SUFFIX_HFQ) {
		base := 1.0
		if exright == byte(types.SUFFIX_QFQ) {
			base = latestHotFactor(exchg, product, rule)
		}
		scaleBarsByHotSections(bars, exchg, product, rule, base, r.adjustFlg)
	}
	// 序列化
	buf := &bytes.Buffer{}
	for i := range bars {
		_ = binary.Write(buf, binary.LittleEndian, &bars[i])
	}
	payload := buf.Bytes()
	cmp := codec.CompressZstd(payload)
	// 头部
	head := make([]byte, 20)
	copy(head[0:8], []byte{'&','^','%','$','#','@','!',0})
	var btype uint16
	switch period {
	case types.KP_Minute1:
		btype = types.BT_HIS_Minute1
	case types.KP_Minute5:
		btype = types.BT_HIS_Minute5
	case types.KP_DAY:
		btype = types.BT_HIS_Day
	default:
		return fmt.Errorf("unsupported period %d", period)
	}
	binary.LittleEndian.PutUint16(head[8:10], btype)
	binary.LittleEndian.PutUint16(head[10:12], uint16(types.BLOCK_VERSION_CMP_V2))
	binary.LittleEndian.PutUint64(head[12:20], uint64(len(cmp)))
	// 输出路径
	name := fmt.Sprintf("%s.%s_%s", exchg, product, rule)
	if exright == byte(types.SUFFIX_QFQ) { name += string([]byte{types.SUFFIX_QFQ}) }
	if exright == byte(types.SUFFIX_HFQ) { name += string([]byte{types.SUFFIX_HFQ}) }
	dir := filepath.Join(r.hisDir, pname, exchg)
	_ = os.MkdirAll(dir, 0o755)
	fn := filepath.Join(dir, name+".dsb")
	log.Printf("[hot] build %s (bars=%d, cmp=%d)", fn, len(bars), len(cmp))
	return os.WriteFile(fn, append(head, cmp...), 0o644)
}