package reader

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// LoadHotSectionsFromFile 读取主连规则配置并注册到内存（与 WT 对齐的两种常见 JSON 结构）
// 支持两种结构：
// 1) exchg -> product -> rule -> [ {code,sdate,edate,factor} ]
// 2) { "rules": [ {exchg,product,rule,sections:[...] } ] }
func (r *Reader) LoadHotSectionsFromFile(path string) error {
	b, err := os.ReadFile(path)
	if err != nil { return err }
	// 尝试结构 2
	var root2 struct{
		Rules []struct{
			Exchg   string        `json:"exchg"`
			Product string        `json:"product"`
			Rule    string        `json:"rule"`
			Sections []struct{
				Code   string  `json:"code"`
				SDate  uint32  `json:"sdate"`
				EDate  uint32  `json:"edate"`
				Factor float64 `json:"factor"`
			} `json:"sections"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(b, &root2); err == nil && len(root2.Rules) > 0 {
		for _, itm := range root2.Rules {
			secs := make([]HotSection, 0, len(itm.Sections))
			for _, s := range itm.Sections {
				secs = append(secs, HotSection{Code:s.Code, SDate:s.SDate, EDate:s.EDate, Factor:s.Factor})
			}
			r.SetCustomRule(itm.Exchg, itm.Product, itm.Rule, secs)
			log.Printf("[hot] loaded %s.%s_%s sections=%d", itm.Exchg, itm.Product, itm.Rule, len(secs))
		}
		return nil
	}
	// 尝试结构 1
	var root1 map[string]map[string]map[string][]struct{
		Code   string  `json:"code"`
		SDate  uint32  `json:"sdate"`
		EDate  uint32  `json:"edate"`
		Factor float64 `json:"factor"`
	}
	if err := json.Unmarshal(b, &root1); err == nil && len(root1) > 0 {
		for exchg, products := range root1 {
			for product, rules := range products {
				for rule, arr := range rules {
					secs := make([]HotSection, 0, len(arr))
					for _, s := range arr {
						secs = append(secs, HotSection{Code:s.Code, SDate:s.SDate, EDate:s.EDate, Factor:s.Factor})
					}
					r.SetCustomRule(exchg, product, rule, secs)
					log.Printf("[hot] loaded %s.%s_%s sections=%d", exchg, product, rule, len(secs))
				}
			}
		}
		return nil
	}
	return fmt.Errorf("unrecognized hot config format: %s", path)
}