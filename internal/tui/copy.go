package tui

import (
	"strings"

	"github.com/MagnumGoYB/aitok/internal/query"
)

type Language string

const (
	LanguageEnglish Language = "en"
	LanguageChinese Language = "zh-CN"
)

type localizedCopy struct {
	title            string
	subtitle         string
	helpCompact      string
	all              string
	search           string
	today            string
	requests         string
	cost             string
	totalTokens      string
	cachedTokens     string
	modelUsage       string
	modelUsageHidden string
	threads          string
	empty            string
	emptyFiltered    string
	help             string
	sortTokens       string
	sortCost         string
	contextTool      string
	contextSort      string
	contextSearch    string
	contextResults   string
	contextThreads   string
	threadDetail     string
	threadLastActive string
	threadSource     string
	threadSplit      string
	threadTokens     string
	copyStatusPrefix string
	headerID         string
	headerName       string
	headerTool       string
	headerModel      string
	headerProvider   string
	headerReq        string
	headerCost       string
	headerSplit      string
	headerPrice      string
	headerTokens     string
	headerInput      string
	headerOutput     string
	headerCached     string
}

func (c localizedCopy) sortBadge(sortBy query.SortMetric) string {
	if normalizePayloadSort(sortBy) == query.SortByCost {
		return "[" + c.sortCost + "]"
	}
	return "[" + c.sortTokens + "]"
}

func copyFor(language Language) localizedCopy {
	if normalizeLanguage(language) == LanguageChinese {
		return localizedCopy{
			title:            "使用统计",
			subtitle:         "查看 AI 模型的使用情况和成本统计",
			helpCompact:      "? help",
			all:              "全部",
			search:           "搜索",
			today:            "当日",
			requests:         "总请求数",
			cost:             "总成本",
			totalTokens:      "总 Token 数",
			cachedTokens:     "缓存 Token",
			modelUsage:       "模型用量",
			modelUsageHidden: "还有 %d 条已折叠；在下方表格滚动查看更多",
			threads:          "会话",
			empty:            "当前查询没有找到用量事件。",
			emptyFiltered:    "当前筛选条件下没有匹配结果。",
			help:             "1 全部 · 2 Claude Code · 3 Codex · 4 Gemini · s 排序 · tab 切换 · j/k 移动 · c 复制 · / 搜索 · l 语言 · q 退出",
			sortTokens:       "按 Tokens",
			sortCost:         "按 Cost",
			contextTool:      "工具",
			contextSort:      "排序",
			contextSearch:    "搜索",
			contextResults:   "模型",
			contextThreads:   "会话",
			threadDetail:     "当前会话",
			threadLastActive: "最近活跃",
			threadSource:     "来源",
			threadSplit:      "拆分",
			threadTokens:     "Tokens",
			copyStatusPrefix: "已复制会话 ID",
			headerID:         "ID",
			headerName:       "名称",
			headerTool:       "工具",
			headerModel:      "模型",
			headerProvider:   "服务商",
			headerReq:        "请求",
			headerCost:       "成本",
			headerSplit:      "拆分",
			headerPrice:      "价格",
			headerTokens:     "Tokens",
			headerInput:      "输入",
			headerOutput:     "输出",
			headerCached:     "缓存",
		}
	}
	return localizedCopy{
		title:            "Usage Dashboard",
		subtitle:         "Monitor AI model usage and estimated cost",
		helpCompact:      "? help",
		all:              "All",
		search:           "Search",
		today:            "Today",
		requests:         "Requests",
		cost:             "Estimated Cost",
		totalTokens:      "Total Tokens",
		cachedTokens:     "Cached Tokens",
		modelUsage:       "Model Usage",
		modelUsageHidden: "%d more folded; scroll the table below to view more",
		threads:          "Threads",
		empty:            "No usage events found for this query.",
		emptyFiltered:    "No rows match the current filters.",
		help:             "1 All · 2 Claude Code · 3 Codex · 4 Gemini · s sort · tab pane · j/k move · c copy · / search · l lang · q quit",
		sortTokens:       "Tokens",
		sortCost:         "Cost",
		contextTool:      "Tool",
		contextSort:      "Sort",
		contextSearch:    "Search",
		contextResults:   "Models",
		contextThreads:   "Threads",
		threadDetail:     "Selected Thread",
		threadLastActive: "Last Active",
		threadSource:     "Source",
		threadSplit:      "Split",
		threadTokens:     "Tokens",
		copyStatusPrefix: "Copied thread ID",
		headerID:         "ID",
		headerName:       "Name",
		headerTool:       "Tool",
		headerModel:      "Model",
		headerProvider:   "Provider",
		headerReq:        "Req",
		headerCost:       "Cost",
		headerSplit:      "Split",
		headerPrice:      "Price",
		headerTokens:     "Tokens",
		headerInput:      "Input",
		headerOutput:     "Output",
		headerCached:     "Cached",
	}
}

func normalizeLanguage(language Language) Language {
	switch Language(strings.ToLower(string(language))) {
	case "zh", "zh-cn", "cn":
		return LanguageChinese
	default:
		return LanguageEnglish
	}
}

func toggleLanguage(language Language) Language {
	if normalizeLanguage(language) == LanguageChinese {
		return LanguageEnglish
	}
	return LanguageChinese
}
