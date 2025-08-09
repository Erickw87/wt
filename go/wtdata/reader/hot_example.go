package reader

// 主连示例配置（仅示例，不会自动加载）
// 对应 WT 的自定义主连规则分段。实际分段应由业务层按实际切换时间生成。

func ExampleRegisterHotSections() {
	// 例如：上期所 rb 品种，规则 main，分三段
	secs := []HotSection{
		{Code: "rb2401", SDate: 20241008, EDate: 20241215, Factor: 1.00},
		{Code: "rb2405", SDate: 20241216, EDate: 20250315, Factor: 0.98},
		{Code: "rb2410", SDate: 20250317, EDate: 20250620, Factor: 1.02},
	}
	AddHotSections("SHFE", "rb", "main", secs)
}