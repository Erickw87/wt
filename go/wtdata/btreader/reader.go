package btreader

// 回测原始数据读取（对应 WtBtDtReader）

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"wtdata/internal/codec"
	"wtdata/internal/types"
)

// Reader 回测文件读取器（对应 WtBtDtReader）
type Reader struct {
	baseDir string
}

// Init 初始化根目录（对应 WtBtDtReader::init，cfg.path）
func (r *Reader) Init(base string) {
	if len(base) == 0 {
		r.baseDir = ""
		return
	}
	// 标准化路径：保证以/结尾
	if base[len(base)-1] != '/' {
		base += "/"
	}
	r.baseDir = base
}

// ReadRawBars 读取历史K线原始块（对应 WtBtDtReader::read_raw_bars）
func (r *Reader) ReadRawBars(exchg, code string, period int) ([]byte, error) {
	pname := types.PERIOD_NAME[period]
	fn := filepath.Join(r.baseDir, "his", pname, exchg, fmt.Sprintf("%s.dsb", code))
	b, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	return codec.ProcBlockData(b, true, false)
}

// ReadRawTicks 读取历史tick原始块（对应 WtBtDtReader::read_raw_ticks）
func (r *Reader) ReadRawTicks(exchg, code string, date uint32) ([]byte, error) {
	fn := filepath.Join(r.baseDir, "his", "ticks", exchg, fmt.Sprintf("%d", date), fmt.Sprintf("%s.dsb", code))
	b, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	return codec.ProcBlockData(b, false, false)
}

// ReadRawOrderDetails 读取历史逐笔委托（对应 WtBtDtReader::read_raw_order_details）
func (r *Reader) ReadRawOrderDetails(exchg, code string, date uint32) ([]byte, error) {
	fn := filepath.Join(r.baseDir, "his", "orders", exchg, fmt.Sprintf("%d", date), fmt.Sprintf("%s.dsb", code))
	b, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	return codec.ProcBlockData(b, false, false)
}

// ReadRawOrderQueues 读取历史委托队列（对应 WtBtDtReader::read_raw_order_queues）
func (r *Reader) ReadRawOrderQueues(exchg, code string, date uint32) ([]byte, error) {
	fn := filepath.Join(r.baseDir, "his", "queue", exchg, fmt.Sprintf("%d", date), fmt.Sprintf("%s.dsb", code))
	b, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	return codec.ProcBlockData(b, false, false)
}

// ReadRawTransactions 读取历史逐笔成交（对应 WtBtDtReader::read_raw_transactions）
func (r *Reader) ReadRawTransactions(exchg, code string, date uint32) ([]byte, error) {
	fn := filepath.Join(r.baseDir, "his", "trans", exchg, fmt.Sprintf("%d", date), fmt.Sprintf("%s.dsb", code))
	b, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	return codec.ProcBlockData(b, false, false)
}