# wtdata (Go)

- Go 重写 WonderTrader `src/WtDataStorage` 模块，保持二进制文件格式与逻辑一致。
- 注释中标明与 WonderTrader 的对应关系，如：`// 对应 WTSTickStruct`。
- 采用 zstd 压缩（对应 WTSCmpHelper 使用的 ZSTD）。

目录
- internal/types: 基础常量与结构体（对应 WTSMarcos.h/WTSStruct.h/DataDefine.h）。
- internal/codec: 块头解析、兼容转换与压缩/解压（对应 proc_block_data/WTSCmpHelper）。
- btreader: 回测原始数据读取（对应 WtBtDtReader）。
- 后续：reader/rdmreader/writer 将逐步补齐。