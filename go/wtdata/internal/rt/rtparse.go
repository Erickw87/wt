package rt

// 实时缓存文件解析（对应 .dmb，DataDefine.h RT*Block）

import (
	"encoding/binary"
	"errors"
	"os"
)

var (
	// 头部大小（RTDayBlockHeader）：BlockHeader(12)+RTBlockHeader扩展(8)+Date(4)=24
	rtDayHeaderSize = 24

	// 元素大小常量（对应新版结构布局大小，单位字节）
	SizeOfBarV2   = 88
	SizeOfTickV2  = 512
	SizeOfOrdQue  = 280
	SizeOfOrdDtl  = 88
	SizeOfTrans   = 104
)

// readRTFile 读取整个 .dmb 文件并返回净荷（去掉头）以及 size/capacity/date/type 信息
func readRTFile(path string) (typ uint16, date uint32, size uint32, capacity uint32, payload []byte, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if len(b) < rtDayHeaderSize {
		err = errors.New("rt file too small")
		return
	}
	// 校验魔数
	if string(b[0:7]) != "&^%$#@" || b[7] != 0 {
		// 保持宽松：不强制报错
	}
	typ = binary.LittleEndian.Uint16(b[8:10])
	// version := binary.LittleEndian.Uint16(b[10:12]) // 预期 RAW_V2
	size = binary.LittleEndian.Uint32(b[12+0 : 12+4])
	capacity = binary.LittleEndian.Uint32(b[12+4 : 12+8])
	date = binary.LittleEndian.Uint32(b[20:24])
	payload = b[rtDayHeaderSize:]
	return
}

// ReadTickBlock 读取实时tick块
func ReadTickBlock(path string) (date uint32, size uint32, payload []byte, err error) {
	typ, dt, sz, capi, pl, e := readRTFile(path)
	if e != nil {
		return 0, 0, nil, e
	}
	_ = capi
	_ = typ // 可校验为 BT_RT_Ticks
	return dt, sz, pl, nil
}

// ReadOrdQueBlock 读取实时委托队列块
func ReadOrdQueBlock(path string) (date uint32, size uint32, payload []byte, err error) {
	_, dt, sz, _, pl, e := readRTFile(path)
	if e != nil {
		return 0, 0, nil, e
	}
	return dt, sz, pl, nil
}

// ReadOrdDtlBlock 读取实时逐笔委托块
func ReadOrdDtlBlock(path string) (date uint32, size uint32, payload []byte, err error) {
	_, dt, sz, _, pl, e := readRTFile(path)
	if e != nil {
		return 0, 0, nil, e
	}
	return dt, sz, pl, nil
}

// ReadTransBlock 读取实时逐笔成交块
func ReadTransBlock(path string) (date uint32, size uint32, payload []byte, err error) {
	_, dt, sz, _, pl, e := readRTFile(path)
	if e != nil {
		return 0, 0, nil, e
	}
	return dt, sz, pl, nil
}

// ReadKlineBlock 读取实时分钟K线块（min1/min5）
func ReadKlineBlock(path string) (date uint32, size uint32, payload []byte, err error) {
	_, dt, sz, _, pl, e := readRTFile(path)
	if e != nil {
		return 0, 0, nil, e
	}
	return dt, sz, pl, nil
}