package types

// 合约与K线等基础类型定义（对应 WTSTypes.h / WTSMarcos.h）

// MAX_EXCHANGE_LENGTH（对应 WTSMarcos.h MAX_EXCHANGE_LENGTH）
const MAX_EXCHANGE_LENGTH = 16

// MAX_INSTRUMENT_LENGTH（对应 WTSMarcos.h MAX_INSTRUMENT_LENGTH）
const MAX_INSTRUMENT_LENGTH = 32

// 复权后缀（对应 Share/CodeHelper.hpp SUFFIX_QFQ / SUFFIX_HFQ）
const (
	SUFFIX_QFQ = '-' // 前复权
	SUFFIX_HFQ = '+' // 后复权
)

// WTSKlinePeriod（对应 WTSTypes.h WTSKlinePeriod）
// 注意：仅存储模块需要 KP_Tick/KP_Minute1/KP_Minute5/KP_DAY
// 其他周期先不实现读取写入
const (
	KP_Tick    = 0
	KP_Minute1 = 1
	KP_Minute5 = 2
	KP_DAY     = 3
	KP_Week    = 4
	KP_Month   = 5
)

// PERIOD_NAME（对应 WTSTypes.h PERIOD_NAME）
var PERIOD_NAME = []string{
	"tick",
	"min1",
	"min5",
	"day",
	"week",
	"month",
}