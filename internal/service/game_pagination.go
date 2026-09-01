package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"xianlv/internal/model"
)

const (
	resultPageCharacterLimit = 2200
	resultPageLineLimit      = 10
	resultPageActionLimit    = 16
)

type cachedGamePage struct {
	Content         string   `json:"content"`
	MarkdownContent string   `json:"markdown_content,omitempty"`
	Actions         []string `json:"actions"`
}

type cachedGameResult struct {
	Title    string           `json:"title"`
	ImageURL string           `json:"image_url"`
	Pages    []cachedGamePage `json:"pages"`
}

func (g *Game) paginateGameResult(player *model.Player, result GameResult) (GameResult, error) {
	content := result.Content
	if strings.TrimSpace(content) == "" {
		content = result.MarkdownContent
	}
	if utf8.RuneCountInString(content) <= resultPageCharacterLimit && lineCount(content) <= resultPageLineLimit && len(result.Actions) <= resultPageActionLimit {
		return result, nil
	}
	contentPages := splitResultContent(content, resultPageCharacterLimit, resultPageLineLimit)
	var markdownPages []string
	if strings.TrimSpace(result.MarkdownContent) != "" {
		markdownPages = splitResultContent(result.MarkdownContent, resultPageCharacterLimit, resultPageLineLimit)
	}
	actionPages := splitResultActions(result.Actions, resultPageActionLimit)
	pageCount := maxInt(maxInt(len(contentPages), len(markdownPages)), len(actionPages))
	if pageCount <= 1 {
		return result, nil
	}
	cache := cachedGameResult{Title: result.Title, ImageURL: result.ImageURL, Pages: make([]cachedGamePage, pageCount)}
	for index := 0; index < pageCount; index++ {
		if index < len(contentPages) {
			cache.Pages[index].Content = contentPages[index]
		}
		if index < len(markdownPages) {
			cache.Pages[index].MarkdownContent = markdownPages[index]
		}
		if index < len(actionPages) {
			cache.Pages[index].Actions = actionPages[index]
		}
	}
	encoded, err := json.Marshal(cache)
	if err != nil {
		return GameResult{}, err
	}
	expires := time.Now().Add(10 * time.Minute)
	if err := g.setPlayerValue(player.ID, "ui.result_pages", string(encoded), &expires); err != nil {
		return GameResult{}, err
	}
	page := cachedPageResult(cache, 1)
	page.BroadcastContent = result.BroadcastContent
	return page, nil
}

func (g *Game) cachedResultPage(player *model.Player, raw string) (GameResult, bool, error) {
	value, err := g.playerValue(player.ID, "ui.result_pages")
	if err != nil {
		return GameResult{Title: "翻页已失效", Content: "上一份长内容已超过十分钟，请重新发送原指令。", Actions: []string{"功能菜单"}}, true, nil
	}
	var cache cachedGameResult
	if json.Unmarshal([]byte(value), &cache) != nil || len(cache.Pages) == 0 {
		return GameResult{Title: "翻页记录损坏", Content: "请重新发送原指令生成列表。", Actions: []string{"功能菜单"}}, true, nil
	}
	page := int(parsePositiveInt(strings.TrimSpace(raw), 1))
	if page > len(cache.Pages) {
		page = len(cache.Pages)
	}
	return cachedPageResult(cache, page), true, nil
}

func cachedPageResult(cache cachedGameResult, page int) GameResult {
	page = maxInt(page, 1)
	if page > len(cache.Pages) {
		page = len(cache.Pages)
	}
	selected := cache.Pages[page-1]
	actions := append([]string(nil), selected.Actions...)
	if page > 1 {
		actions = append(actions, fmt.Sprintf("翻页 %d", page-1))
	}
	if page < len(cache.Pages) {
		actions = append(actions, fmt.Sprintf("翻页 %d", page+1))
	}
	content := strings.TrimSpace(selected.Content)
	if content == "" {
		content = "本页为操作指令续页，请点击下方蓝字。"
	}
	content += fmt.Sprintf("\n━━━━━━━\n第%d/%d页 · 长内容保留十分钟", page, len(cache.Pages))
	markdown := strings.TrimSpace(selected.MarkdownContent)
	if markdown != "" {
		markdown += fmt.Sprintf("\n━━━━━━━\n第%d/%d页 · 长内容保留十分钟", page, len(cache.Pages))
	}
	return GameResult{Title: cache.Title, Content: content, MarkdownContent: markdown, ImageURL: cache.ImageURL, Actions: actions}
}

func splitResultContent(content string, characterLimit, lineLimit int) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return []string{""}
	}
	var pages []string
	var current strings.Builder
	currentLines := 0
	flush := func() {
		if current.Len() == 0 {
			return
		}
		pages = append(pages, strings.TrimSpace(current.String()))
		current.Reset()
		currentLines = 0
	}
	for _, line := range strings.Split(content, "\n") {
		lineRunes := []rune(line)
		for len(lineRunes) > characterLimit {
			flush()
			pages = append(pages, string(lineRunes[:characterLimit]))
			lineRunes = lineRunes[characterLimit:]
		}
		line = string(lineRunes)
		addition := utf8.RuneCountInString(line)
		if current.Len() > 0 {
			addition++
		}
		if currentLines >= lineLimit || utf8.RuneCountInString(current.String())+addition > characterLimit {
			flush()
		}
		if current.Len() > 0 {
			current.WriteByte('\n')
		}
		current.WriteString(line)
		currentLines++
	}
	flush()
	return pages
}

func lineCount(content string) int {
	content = strings.TrimSpace(content)
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

func splitResultActions(actions []string, limit int) [][]string {
	if len(actions) == 0 {
		return nil
	}
	pages := make([][]string, 0, (len(actions)+limit-1)/limit)
	for start := 0; start < len(actions); start += limit {
		end := minInt(start+limit, len(actions))
		pages = append(pages, append([]string(nil), actions[start:end]...))
	}
	return pages
}
