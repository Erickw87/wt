package types

// Bar/K线与逐笔等基础结构体（对应 WTSStruct.h 新版结构体）

// WTSBarStruct（对应 WTSBarStruct）
// 注意：WT 使用 #pragma pack(push,8)，Go 默认按本地对齐。这里用于磁盘读写时将采用二进制编解码而非直接内存映射，确保字节一致。
type WTSBarStruct struct {
	Date    uint32  // 日期（对应 WTSBarStruct.date）
	Reserve uint32  // 占位符（对应 WTSBarStruct.reserve_）
	Time    uint64  // 时间（对应 WTSBarStruct.time）
	Open    float64 // 开
	High    float64 // 高
	Low     float64 // 低
	Close   float64 // 收
	Settle  float64 // 结算
	Money   float64 // 成交金额
	Vol     float64 // 成交量
	Hold    float64 // 总持（与 bid 共用 union，Go 单字段表示；min_price_mode=1 时语义为买价）
	Add     float64 // 增仓（与 ask 共用 union，Go 单字段表示；min_price_mode=1 时语义为卖价）
}

// WTSTickStruct（对应 WTSTickStruct）
type WTSTickStruct struct {
	Exchg         [MAX_EXCHANGE_LENGTH]byte      // 交易所（对应 WTSTickStruct.exchg）
	Code          [MAX_INSTRUMENT_LENGTH]byte    // 合约代码（对应 WTSTickStruct.code）
	Price         float64                        // 最新价
	Open          float64                        // 开盘价
	High          float64                        // 最高价
	Low           float64                        // 最低价
	SettlePrice   float64                        // 结算价
	UpperLimit    float64                        // 涨停价
	LowerLimit    float64                        // 跌停价
	TotalVolume   float64                        // 总成交量
	Volume        float64                        // 成交量
	TotalTurnover float64                        // 总成交额
	TurnOver      float64                        // 成交额
	OpenInterest  float64                        // 总持
	DiffInterest  float64                        // 增仓
	TradingDate   uint32                         // 交易日
	ActionDate    uint32                         // 自然日
	ActionTime    uint32                         // 发生时间（毫秒）
	Reserve       uint32                         // 占位符
	PreClose      float64                        // 昨收
	PreSettle     float64                        // 昨结算
	PreInterest   float64                        // 上日总持
	BidPrices     [10]float64                    // 委买价
	AskPrices     [10]float64                    // 委卖价
	BidQty        [10]float64                    // 委买量（WT 为 double[10]）
	AskQty        [10]float64                    // 委卖量（WT 为 double[10]）
}

// WTSOrdQueStruct（对应 WTSOrdQueStruct）
type WTSOrdQueStruct struct {
	Exchg       [MAX_EXCHANGE_LENGTH]byte
	Code        [MAX_INSTRUMENT_LENGTH]byte
	TradingDate uint32
	ActionDate  uint32
	ActionTime  uint32
	Side        uint32   // WTSBSDirectType（对应 WTSTypes.h 定义）
	Price       float64
	OrderItems  uint32
	QSize       uint32
	Volumes     [50]uint32
}

// WTSOrdDtlStruct（对应 WTSOrdDtlStruct）
type WTSOrdDtlStruct struct {
	Exchg       [MAX_EXCHANGE_LENGTH]byte
	Code        [MAX_INSTRUMENT_LENGTH]byte
	TradingDate uint32
	ActionDate  uint32
	ActionTime  uint32
	Index       uint64
	Price       float64
	Volume      uint32
	Side        uint32 // WTSBSDirectType
	OType       uint32 // WTSOrdDetailType（对应 WTSTypes.h）
}

// WTSTransStruct（对应 WTSTransStruct）
type WTSTransStruct struct {
	Exchg       [MAX_EXCHANGE_LENGTH]byte
	Code        [MAX_INSTRUMENT_LENGTH]byte
	TradingDate uint32
	ActionDate  uint32
	ActionTime  uint32
	Index       int64
	TType       uint32 // WTSTransType（对应 WTSTypes.h）
	Side        uint32 // WTSBSDirectType
	Price       float64
	Volume      uint32
	AskOrder    int64
	BidOrder    int64
}