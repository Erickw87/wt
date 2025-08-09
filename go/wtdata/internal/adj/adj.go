package adj

// 除权因子管理（对应 WtDataReader::loadStkAdjFactorsFromFile / getAdjFactorByDate）

import (
	"encoding/json"
	"os"
	"sort"
	"fmt"
)

type Factor struct {
	Date   uint32  `json:"date"`
	Factor float64 `json:"factor"`
}

type Map = map[string][]Factor // key: exchg.PID.code （PID 默认 STK）

// LoadFromFile 读取 adjfactor JSON，结构参考 WT：顶层为 exchg -> code -> [{date,factor},...]
// 若 code 中已包含 PID（如 STK.600000），则 key 为 exchg.code；否则使用 exchg.STK.code
func LoadFromFile(path string) (Map, error) {
	b, err := os.ReadFile(path)
	if err != nil { return nil, err }
	var root map[string]map[string][]Factor
	if err := json.Unmarshal(b, &root); err != nil { return nil, err }
	res := Map{}
	for exchg, codes := range root {
		for code, arr := range codes {
			key := code
			if !containsDot(code) {
				key = fmt.Sprintf("STK.%s", code)
			}
			full := fmt.Sprintf("%s.%s", exchg, key)
			// 追加一个基准因子（19900101,1）
			list := make([]Factor, 0, len(arr)+1)
			list = append(list, arr...)
			list = append(list, Factor{Date:19900101, Factor:1})
			sort.Slice(list, func(i, j int) bool { return list[i].Date < list[j].Date })
			res[full] = list
		}
	}
	return res, nil
}

func containsDot(s string) bool {
	for i := 0; i < len(s); i++ { if s[i]=='.' { return true } }
	return false
}

// GetFactorByDate 获取指定日期的因子（等价 lower_bound，命中大于目标时取上一条；越界则返回最后一条）
func GetFactorByDate(m Map, key string, date uint32) float64 {
	lst := m[key]
	if len(lst) == 0 { return 1 }
	// 二分
	l, h := 0, len(lst)-1
	idx := -1
	for l <= h {
		mid := (l+h)>>1
		if lst[mid].Date <= date { idx = mid; l = mid+1 } else { h = mid-1 }
	}
	if idx < 0 { return 1 }
	return lst[idx].Factor
}

// GetLastFactor 获取最后一条因子
func GetLastFactor(m Map, key string) float64 {
	lst := m[key]
	if len(lst)==0 { return 1 }
	return lst[len(lst)-1].Factor
}