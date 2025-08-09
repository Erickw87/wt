package codec

// 旧版结构转换到新版（对应 WTSBarStructOld/WTSTickStructOld -> WTSBarStruct/WTSTickStruct）

import (
	"encoding/binary"
	"errors"
	"math"
)

// BarOldToNew 将旧版bar字节序列转换为新版bar字节序列
// 输入为不含块头的纯数组区，输出为按新版布局的字节流（连续的 WTSBarStruct）。
func BarOldToNew(oldPayload []byte) ([]byte, error) {
	const barOldSize = 4 + 4 + 8*5 + 4 + 4 + 4 // date,time, open,high,low,close,settle,money, vol,hold,add
	if len(oldPayload)%barOldSize != 0 {
		return nil, errors.New("invalid old bar payload size")
	}
	cnt := len(oldPayload) / barOldSize
	// 新版大小：date(4)+reserve(4)+time(8)+8*8 + ... 共：4+4+8 + 8*8? 实际字段数= open,high,low,close,settle,money,vol,hold,add 共9个float64 => 9*8=72; 4+4+8+72=88
	const barNewSize = 4 + 4 + 8 + 8*9
	out := make([]byte, cnt*barNewSize)
	for i := 0; i < cnt; i++ {
		offOld := i * barOldSize
		offNew := i * barNewSize
		date := binary.LittleEndian.Uint32(oldPayload[offOld:])
		time := binary.LittleEndian.Uint32(oldPayload[offOld+4:])
		open := binary.LittleEndian.Uint64(oldPayload[offOld+8:])
		high := binary.LittleEndian.Uint64(oldPayload[offOld+16:])
		low := binary.LittleEndian.Uint64(oldPayload[offOld+24:])
		closep := binary.LittleEndian.Uint64(oldPayload[offOld+32:])
		settle := binary.LittleEndian.Uint64(oldPayload[offOld+40:])
		money := binary.LittleEndian.Uint64(oldPayload[offOld+48:])
		vol := binary.LittleEndian.Uint32(oldPayload[offOld+56:])
		hold := binary.LittleEndian.Uint32(oldPayload[offOld+60:])
		add := binary.LittleEndian.Uint32(oldPayload[offOld+64:])

		// 写入新版
		binary.LittleEndian.PutUint32(out[offNew:], date)
		binary.LittleEndian.PutUint32(out[offNew+4:], 0) // reserve_
		binary.LittleEndian.PutUint64(out[offNew+8:], uint64(time))
		binary.LittleEndian.PutUint64(out[offNew+16:], open)
		binary.LittleEndian.PutUint64(out[offNew+24:], high)
		binary.LittleEndian.PutUint64(out[offNew+32:], low)
		binary.LittleEndian.PutUint64(out[offNew+40:], closep)
		binary.LittleEndian.PutUint64(out[offNew+48:], settle)
		binary.LittleEndian.PutUint64(out[offNew+56:], money)
		binary.LittleEndian.PutUint64(out[offNew+64:], uint64(vol))
		binary.LittleEndian.PutUint64(out[offNew+72:], uint64(hold))
		binary.LittleEndian.PutUint64(out[offNew+80:], uint64(add))
	}
	return out, nil
}

// TickOldToNew 将旧版tick字节序列转换为新版tick字节序列
func TickOldToNew(oldPayload []byte) ([]byte, error) {
	// 旧版布局：exchg[10], code[32], 5*8(价格) + 2*8(上下限) + vol,totalVol(2*4?), 实际旧版中 total_volume(uint32), volume(uint32), total_turnover(double), turn_over(double), open_interest(uint32), diff_interest(int32),
	// 后续日期时间3*4，pre_close/pre_settle/diff_interest(8,8,4)，之后 10*(price*8) + 10*(ask*8) + 10*(bid_qty*4) + 10*(ask_qty*4)
	// 为避免误差，逐字段读取并写入新版布局
	// 计算最少长度检查
	minSize := 10 + 32 + 5*8 + 2*8 + 4 + 4 + 8 + 8 + 4 + 4 + 4 + 4 + 4 + 10*8 + 10*8 + 10*4 + 10*4
	if len(oldPayload)%minSize != 0 {
		return nil, errors.New("invalid old tick payload size")
	}
	cnt := len(oldPayload) / minSize
	// 新版 tick 大致大小：exchg[16]+code[32]+14*8 + 4*4 + 3*8 + 40*8
	newSize := 16 + 32 + 14*8 + 4*4 + 3*8 + 40*8
	out := make([]byte, cnt*newSize)
	for i := 0; i < cnt; i++ {
		offOld := i * minSize
		offNew := i * newSize
		p := offOld
		// exchg[<=10] -> 16 bytes
		copy(out[offNew:offNew+16], make([]byte, 16))
		copy(out[offNew:], oldPayload[p:p+10])
		p += 10
		// code[<=32] -> 32 bytes
		copy(out[offNew+16:offNew+48], oldPayload[p:p+32])
		p += 32
		// doubles
		copy(out[offNew+48:offNew+48+8*5], oldPayload[p:p+8*5]) // price,open,high,low,settle_price
		p += 8 * 5
		copy(out[offNew+88:offNew+88+16], oldPayload[p:p+16]) // upper_limit,lower_limit
		p += 16
		// total_volume(uint32) -> float64, volume(uint32) -> float64
		totalVol := float64(binary.LittleEndian.Uint32(oldPayload[p:]))
		p += 4
		vol := float64(binary.LittleEndian.Uint32(oldPayload[p:]))
		p += 4
		binary.LittleEndian.PutUint64(out[offNew+104:], math.Float64bits(totalVol))
		binary.LittleEndian.PutUint64(out[offNew+112:], math.Float64bits(vol))
		// total_turnover(double), turn_over(double)
		copy(out[offNew+120:offNew+136], oldPayload[p:p+16])
		p += 16
		// open_interest(uint32)->float64, diff_interest(int32)->float64
		openInt := float64(binary.LittleEndian.Uint32(oldPayload[p:]))
		p += 4
		diffInt := float64(int32(binary.LittleEndian.Uint32(oldPayload[p:])))
		p += 4
		binary.LittleEndian.PutUint64(out[offNew+136:], math.Float64bits(openInt))
		binary.LittleEndian.PutUint64(out[offNew+144:], math.Float64bits(diffInt))
		// dates
		copy(out[offNew+152:offNew+164], oldPayload[p:p+12]) // trading_date,action_date,action_time
		p += 12
		// reserve_ 默认 0（4 字节）
		// pre_close, pre_settle, pre_interest(int32->float64)
		copy(out[offNew+168:offNew+184], oldPayload[p:p+16])
		p += 16
		preInt := float64(int32(binary.LittleEndian.Uint32(oldPayload[p:])))
		p += 4
		binary.LittleEndian.PutUint64(out[offNew+184:], math.Float64bits(preInt))
		// bid_prices[10], ask_prices[10]
		copy(out[offNew+192:offNew+192+10*8], oldPayload[p:p+10*8])
		p += 10 * 8
		copy(out[offNew+272:offNew+272+10*8], oldPayload[p:p+10*8])
		p += 10 * 8
		// bid_qty[10] uint32 -> float64[10]
		for k := 0; k < 10; k++ {
			q := float64(binary.LittleEndian.Uint32(oldPayload[p:]))
			binary.LittleEndian.PutUint64(out[offNew+352+k*8:], math.Float64bits(q))
			p += 4
		}
		// ask_qty
		for k := 0; k < 10; k++ {
			q := float64(binary.LittleEndian.Uint32(oldPayload[p:]))
			binary.LittleEndian.PutUint64(out[offNew+432+k*8:], math.Float64bits(q))
			p += 4
		}
	}
	return out, nil
}

func mathFloat64bits(f float64) uint64 { return binary.LittleEndian.Uint64((&[8]byte{})[:]) }