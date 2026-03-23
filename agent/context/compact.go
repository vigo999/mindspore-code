package context

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vigo999/ms-cli/integrations/llm"
)

// CompactStrategy 压缩策略类型
type CompactStrategy int

const (
	// CompactStrategySimple 简单策略：直接丢弃旧消息
	CompactStrategySimple CompactStrategy = iota
	// CompactStrategySummarize 摘要策略：将旧消息摘要为一句话
	CompactStrategySummarize
	// CompactStrategyPriority 优先级策略：基于优先级保留消息
	CompactStrategyPriority
	// CompactStrategyHybrid 混合策略：结合多种策略
	CompactStrategyHybrid
)

// String 返回策略名称
func (s CompactStrategy) String() string {
	switch s {
	case CompactStrategySimple:
		return "simple"
	case CompactStrategySummarize:
		return "summarize"
	case CompactStrategyPriority:
		return "priority"
	case CompactStrategyHybrid:
		return "hybrid"
	default:
		return "unknown"
	}
}

// ParseCompactStrategy 解析策略字符串
func ParseCompactStrategy(s string) CompactStrategy {
	switch strings.ToLower(s) {
	case "simple":
		return CompactStrategySimple
	case "summarize":
		return CompactStrategySummarize
	case "priority":
		return CompactStrategyPriority
	case "hybrid":
		return CompactStrategyHybrid
	default:
		return CompactStrategySimple
	}
}

// Compactor 上下文压缩器
type Compactor struct {
	strategy        CompactStrategy
	scorer          *PriorityScorer
	tokenizer       *Tokenizer
	maxKeepMessages int // 最大保留消息数
}

type messageGroup struct {
	Messages []llm.Message
	Start    int
}

type prioritizedGroup struct {
	Group    messageGroup
	Priority Priority
}

// CompactorConfig 压缩器配置
type CompactorConfig struct {
	Strategy        CompactStrategy
	MaxKeepMessages int
}

// NewCompactor 创建新的压缩器
func NewCompactor(cfg CompactorConfig) *Compactor {
	if cfg.MaxKeepMessages <= 0 {
		cfg.MaxKeepMessages = 20
	}
	return &Compactor{
		strategy:        cfg.Strategy,
		scorer:          NewPriorityScorer(),
		tokenizer:       NewTokenizer(),
		maxKeepMessages: cfg.MaxKeepMessages,
	}
}

// SetStrategy 设置压缩策略
func (c *Compactor) SetStrategy(s CompactStrategy) {
	c.strategy = s
}

// Compact 执行压缩
func (c *Compactor) Compact(messages []llm.Message, systemMsg *llm.Message) ([]llm.Message, CompactResult) {
	if len(messages) <= c.maxKeepMessages {
		return messages, CompactResult{Kept: len(messages), Removed: 0}
	}

	switch c.strategy {
	case CompactStrategySimple:
		return c.compactSimple(messages, systemMsg)
	case CompactStrategySummarize:
		return c.compactSummarize(messages, systemMsg)
	case CompactStrategyPriority:
		return c.compactPriority(messages, systemMsg)
	case CompactStrategyHybrid:
		return c.compactHybrid(messages, systemMsg)
	default:
		return c.compactSimple(messages, systemMsg)
	}
}

// compactSimple 简单压缩策略
func (c *Compactor) compactSimple(messages []llm.Message, systemMsg *llm.Message) ([]llm.Message, CompactResult) {
	// 保留最近的消息
	keepCount := c.maxKeepMessages
	if systemMsg != nil {
		keepCount-- // 为系统消息留一个位置
	}

	if keepCount >= len(messages) {
		return messages, CompactResult{Kept: len(messages), Removed: 0}
	}

	result := keepRecentMessages(messages, keepCount)
	removed := len(messages) - len(result)

	return result, CompactResult{
		Kept:     len(result),
		Removed:  removed,
		Strategy: CompactStrategySimple,
		Summary:  fmt.Sprintf("Removed %d old messages", removed),
	}
}

// compactSummarize 摘要压缩策略
func (c *Compactor) compactSummarize(messages []llm.Message, systemMsg *llm.Message) ([]llm.Message, CompactResult) {
	// 保留最近的消息
	keepCount := c.maxKeepMessages - 2 // 留出位置给摘要和系统消息
	if keepCount < 4 {
		keepCount = 4
	}

	if keepCount >= len(messages) {
		return messages, CompactResult{Kept: len(messages), Removed: 0}
	}

	groups := groupMessages(messages)
	keptGroups := keepRecentMessageGroups(groups, keepCount)
	toSummarize := flattenMessageGroups(excludeMessageGroups(groups, keptGroups))
	if len(toSummarize) == 0 {
		kept := flattenMessageGroups(keptGroups)
		return kept, CompactResult{
			Kept:     len(kept),
			Removed:  0,
			Strategy: CompactStrategySummarize,
			Summary:  "No messages summarized",
		}
	}

	// 生成摘要
	summary := c.generateSummary(toSummarize)
	summaryMsg := llm.NewSystemMessage(summary)

	// 保留的消息
	result := append([]llm.Message{summaryMsg}, flattenMessageGroups(keptGroups)...)

	return result, CompactResult{
		Kept:     len(result),
		Removed:  len(toSummarize),
		Strategy: CompactStrategySummarize,
		Summary:  summary,
	}
}

// compactPriority 优先级压缩策略
func (c *Compactor) compactPriority(messages []llm.Message, systemMsg *llm.Message) ([]llm.Message, CompactResult) {
	prioritized := c.prioritizeGroups(groupMessages(messages), len(messages))

	// 保留优先级最高的消息组
	keepCount := c.maxKeepMessages
	if systemMsg != nil {
		keepCount--
	}
	result := flattenMessageGroups(selectPrioritizedGroups(prioritized, keepCount))

	return result, CompactResult{
		Kept:     len(result),
		Removed:  len(messages) - len(result),
		Strategy: CompactStrategyPriority,
		Summary:  fmt.Sprintf("Kept %d high-priority messages, removed %d", len(result), len(messages)-len(result)),
	}
}

// compactHybrid 混合压缩策略
func (c *Compactor) compactHybrid(messages []llm.Message, systemMsg *llm.Message) ([]llm.Message, CompactResult) {
	// 策略：
	// 1. 保留最近的几条消息（高优先级）
	// 2. 基于优先级选择保留的较旧消息
	// 3. 将其他旧消息摘要

	recentCount := c.maxKeepMessages / 2 // 保留一半给最新消息
	if recentCount < 3 {
		recentCount = 3
	}

	if len(messages) <= c.maxKeepMessages {
		return messages, CompactResult{Kept: len(messages), Removed: 0}
	}

	groups := groupMessages(messages)
	recentGroups := keepRecentMessageGroups(groups, recentCount)
	oldGroups := excludeMessageGroups(groups, recentGroups)
	prioritized := c.prioritizeGroups(oldGroups, len(messages))

	// 保留高优先级的旧消息
	oldKeepCount := c.maxKeepMessages - recentCount - 1 // 留出位置给摘要
	if oldKeepCount < 0 {
		oldKeepCount = 0
	}

	var result []llm.Message
	highPriorityOld := selectPrioritizedGroups(prioritized, oldKeepCount)

	// 如果有需要摘要的旧消息，添加摘要
	if len(oldGroups) > len(highPriorityOld) {
		toSummarize := flattenMessageGroups(excludeMessageGroups(oldGroups, highPriorityOld))
		if len(toSummarize) > 0 {
			summary := c.generateSummary(toSummarize)
			result = append(result, llm.NewSystemMessage(summary))
		}
	}

	// 添加保留的高优先级旧消息
	if len(highPriorityOld) > 0 {
		result = append(result, flattenMessageGroups(highPriorityOld)...)
	}

	// 添加最近的消息
	result = append(result, flattenMessageGroups(recentGroups)...)

	removed := len(messages) - len(result)

	return result, CompactResult{
		Kept:     len(result),
		Removed:  removed,
		Strategy: CompactStrategyHybrid,
		Summary:  fmt.Sprintf("Hybrid compact: kept %d messages including %d recent", len(result), recentCount),
	}
}

// generateSummary 生成消息摘要
func (c *Compactor) generateSummary(messages []llm.Message) string {
	userCount := 0
	assistantCount := 0
	toolCount := 0

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			userCount++
		case "assistant":
			assistantCount++
		case "tool":
			toolCount++
		}
	}

	parts := []string{"[Context Summary]"}
	parts = append(parts, fmt.Sprintf("Earlier conversation: %d messages", len(messages)))

	if userCount > 0 {
		parts = append(parts, fmt.Sprintf("%d user messages", userCount))
	}
	if assistantCount > 0 {
		parts = append(parts, fmt.Sprintf("%d assistant responses", assistantCount))
	}
	if toolCount > 0 {
		parts = append(parts, fmt.Sprintf("%d tool calls", toolCount))
	}

	return strings.Join(parts, ", ")
}

// CompactResult 压缩结果
type CompactResult struct {
	Kept     int
	Removed  int
	Strategy CompactStrategy
	Summary  string
}

// String 返回压缩结果的字符串表示
func (r CompactResult) String() string {
	return fmt.Sprintf("Compact [%s]: kept %d, removed %d - %s",
		r.Strategy, r.Kept, r.Removed, r.Summary)
}

// SimpleCompact 简单压缩函数（保持向后兼容）
func SimpleCompact(messages []llm.Message, maxKeep int) []llm.Message {
	if len(messages) <= maxKeep {
		return messages
	}
	return keepRecentMessages(messages, maxKeep)
}

func groupMessages(messages []llm.Message) []messageGroup {
	if len(messages) == 0 {
		return nil
	}

	groups := make([]messageGroup, 0, len(messages))
	for i := 0; i < len(messages); {
		start := i
		i++
		if messages[start].Role == "assistant" && len(messages[start].ToolCalls) > 0 {
			for i < len(messages) && messages[i].Role == "tool" {
				i++
			}
		}
		groups = append(groups, messageGroup{
			Messages: messages[start:i],
			Start:    start,
		})
	}

	return groups
}

func flattenMessageGroups(groups []messageGroup) []llm.Message {
	if len(groups) == 0 {
		return nil
	}

	total := 0
	for _, group := range groups {
		total += len(group.Messages)
	}

	flattened := make([]llm.Message, 0, total)
	for _, group := range groups {
		flattened = append(flattened, group.Messages...)
	}
	return flattened
}

func keepRecentMessages(messages []llm.Message, maxKeep int) []llm.Message {
	if maxKeep <= 0 {
		return nil
	}
	return flattenMessageGroups(keepRecentMessageGroups(groupMessages(messages), maxKeep))
}

func keepRecentMessageGroups(groups []messageGroup, maxKeep int) []messageGroup {
	if len(groups) == 0 || maxKeep <= 0 {
		return nil
	}

	pinned := pinnedMessageGroups(groups)
	pinnedSet := messageGroupSet(pinned)
	kept := append([]messageGroup{}, pinned...)
	remainingBudget := maxKeep - countMessagesInGroups(pinned)
	if remainingBudget <= 0 {
		sortMessageGroupsByStart(kept)
		return kept
	}

	recent := make([]messageGroup, 0, len(groups))
	keptMessages := 0
	for i := len(groups) - 1; i >= 0; i-- {
		if _, ok := pinnedSet[groups[i].Start]; ok {
			continue
		}
		groupSize := len(groups[i].Messages)
		if keptMessages+groupSize > remainingBudget && len(recent) > 0 {
			break
		}
		recent = append(recent, groups[i])
		keptMessages += groupSize
		if keptMessages >= remainingBudget {
			break
		}
	}

	kept = append(kept, recent...)
	sortMessageGroupsByStart(kept)
	return kept
}

func (c *Compactor) prioritizeGroups(groups []messageGroup, totalMessages int) []prioritizedGroup {
	prioritized := make([]prioritizedGroup, 0, len(groups))
	for _, group := range groups {
		priority := PriorityLow
		for offset, msg := range group.Messages {
			score := c.scorer.ScoreMessage(msg, group.Start+offset, totalMessages)
			if score > priority {
				priority = score
			}
		}
		prioritized = append(prioritized, prioritizedGroup{
			Group:    group,
			Priority: priority,
		})
	}

	sort.Slice(prioritized, func(i, j int) bool {
		if prioritized[i].Priority == prioritized[j].Priority {
			return prioritized[i].Group.Start > prioritized[j].Group.Start
		}
		return prioritized[i].Priority > prioritized[j].Priority
	})
	return prioritized
}

func selectPrioritizedGroups(groups []prioritizedGroup, maxKeep int) []messageGroup {
	if len(groups) == 0 {
		return nil
	}

	pinned := make([]messageGroup, 0)
	pinnedSet := make(map[int]struct{})
	for _, group := range groups {
		if !isPinnedMessageGroup(group.Group) {
			continue
		}
		pinned = append(pinned, group.Group)
		pinnedSet[group.Group.Start] = struct{}{}
	}

	selected := append([]messageGroup{}, pinned...)
	remainingBudget := maxKeep - countMessagesInGroups(pinned)
	if remainingBudget <= 0 {
		sortMessageGroupsByStart(selected)
		return selected
	}

	keptMessages := 0
	for _, group := range groups {
		if _, ok := pinnedSet[group.Group.Start]; ok {
			continue
		}
		groupSize := len(group.Group.Messages)
		if keptMessages+groupSize > remainingBudget && len(selected) > len(pinned) {
			continue
		}
		selected = append(selected, group.Group)
		keptMessages += groupSize
		if keptMessages >= remainingBudget {
			break
		}
	}

	sortMessageGroupsByStart(selected)
	return selected
}

func excludeMessageGroups(groups, excluded []messageGroup) []messageGroup {
	if len(groups) == 0 {
		return nil
	}
	if len(excluded) == 0 {
		result := make([]messageGroup, len(groups))
		copy(result, groups)
		return result
	}

	excludedSet := messageGroupSet(excluded)
	result := make([]messageGroup, 0, len(groups))
	for _, group := range groups {
		if _, ok := excludedSet[group.Start]; ok {
			continue
		}
		result = append(result, group)
	}
	return result
}

func pinnedMessageGroups(groups []messageGroup) []messageGroup {
	result := make([]messageGroup, 0)
	for _, group := range groups {
		if isPinnedMessageGroup(group) {
			result = append(result, group)
		}
	}
	return result
}

func isPinnedMessageGroup(group messageGroup) bool {
	for _, msg := range group.Messages {
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name == "load_skill" {
				return true
			}
		}
	}
	return false
}

func countMessagesInGroups(groups []messageGroup) int {
	total := 0
	for _, group := range groups {
		total += len(group.Messages)
	}
	return total
}

func messageGroupSet(groups []messageGroup) map[int]struct{} {
	result := make(map[int]struct{}, len(groups))
	for _, group := range groups {
		result[group.Start] = struct{}{}
	}
	return result
}

func sortMessageGroupsByStart(groups []messageGroup) {
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Start < groups[j].Start
	})
}
