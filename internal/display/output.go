package display

import (
	"fmt"
	"strings"
	"sync"

	"github.com/andreassag/zotero-tagger/internal/zotero"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginTop(1)

	subHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	orgTagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))

	groupTagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	topicTagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141"))

	sourceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220"))
)

type Output struct {
	mu sync.Mutex
}

func NewOutput() *Output {
	return &Output{}
}

func (o *Output) RenderItemHeader(title, key string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	fmt.Println(headerStyle.Render(fmt.Sprintf("📄 Paper: %s", title)))
	fmt.Println(subHeaderStyle.Render(fmt.Sprintf("   Key: %s", key)))
}

func (o *Output) RenderItemSource(sourceType, details string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if details != "" {
		fmt.Println(sourceStyle.Render(fmt.Sprintf("   📎 Source: %s (%s)", sourceType, details)))
	} else {
		fmt.Println(sourceStyle.Render(fmt.Sprintf("   📎 Source: %s", sourceType)))
	}
}

func (o *Output) RenderTags(tags []zotero.Tag) {
	o.mu.Lock()
	defer o.mu.Unlock()

	var orgs, groups, topics []string

	for _, t := range tags {
		if strings.HasPrefix(t.Tag, "org:") {
			orgs = append(orgs, orgTagStyle.Render(t.Tag))
		} else if strings.HasPrefix(t.Tag, "group:") {
			groups = append(groups, groupTagStyle.Render(t.Tag))
		} else if strings.HasPrefix(t.Tag, "topic:") {
			topics = append(topics, topicTagStyle.Render(t.Tag))
		}
	}

	fmt.Println("   🏷️  Assigned Tags:")
	if len(orgs) > 0 {
		fmt.Printf("      Orgs:   %s\n", strings.Join(orgs, ", "))
	}
	if len(groups) > 0 {
		fmt.Printf("      Groups: %s\n", strings.Join(groups, ", "))
	}
	if len(topics) > 0 {
		fmt.Printf("      Topics: %s\n", strings.Join(topics, ", "))
	}
	fmt.Println()
}

func (o *Output) RenderItemResult(title, key, sourceType, sourceDetails string, tags []zotero.Tag) {
	o.mu.Lock()
	defer o.mu.Unlock()

	fmt.Println(headerStyle.Render(fmt.Sprintf("📄 Paper: %s", title)))
	fmt.Println(subHeaderStyle.Render(fmt.Sprintf("   Key: %s", key)))

	if sourceDetails != "" {
		fmt.Println(sourceStyle.Render(fmt.Sprintf("   📎 Source: %s (%s)", sourceType, sourceDetails)))
	} else if sourceType != "" {
		fmt.Println(sourceStyle.Render(fmt.Sprintf("   📎 Source: %s", sourceType)))
	}

	var orgs, groups, topics []string
	for _, t := range tags {
		if strings.HasPrefix(t.Tag, "org:") {
			orgs = append(orgs, orgTagStyle.Render(t.Tag))
		} else if strings.HasPrefix(t.Tag, "group:") {
			groups = append(groups, groupTagStyle.Render(t.Tag))
		} else if strings.HasPrefix(t.Tag, "topic:") {
			topics = append(topics, topicTagStyle.Render(t.Tag))
		}
	}

	fmt.Println("   🏷️  Assigned Tags:")
	if len(orgs) > 0 {
		fmt.Printf("      Orgs:   %s\n", strings.Join(orgs, ", "))
	}
	if len(groups) > 0 {
		fmt.Printf("      Groups: %s\n", strings.Join(groups, ", "))
	}
	if len(topics) > 0 {
		fmt.Printf("      Topics: %s\n", strings.Join(topics, ", "))
	}
	fmt.Println()
}

func (o *Output) RenderSummary(total, processed, skipped, errors int) {
	o.mu.Lock()
	defer o.mu.Unlock()

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("99"))).
		Headers("Metric", "Count").
		Row("Total Items Evaluated", fmt.Sprintf("%d", total)).
		Row("Successfully Processed", fmt.Sprintf("%d", processed)).
		Row("Skipped Items", fmt.Sprintf("%d", skipped)).
		Row("Errors Encountered", fmt.Sprintf("%d", errors))

	fmt.Println(headerStyle.Render("\n🏁 Tagging Pipeline Run Complete"))
	fmt.Println(t)
}
