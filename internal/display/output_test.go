package display

import (
	"sync"
	"testing"

	"github.com/andreassag/zotero-tagger/internal/zotero"
	"github.com/stretchr/testify/assert"
)

func TestOutput_RenderMethods(t *testing.T) {
	out := NewOutput()
	assert.NotNil(t, out)

	assert.NotPanics(t, func() {
		out.RenderItemHeader("Test Title", "ABC12345")
		out.RenderItemSource("PDF Document", "1.5 MB")
		out.RenderItemSource("Abstract Note", "")
		out.RenderTags([]zotero.Tag{
			{Tag: "org:escherichia-coli"},
			{Tag: "group:enterobacteriaceae"},
			{Tag: "topic:biofilm"},
			{Tag: "_ai-tagged"},
		})
		out.RenderItemResult("Test Title", "ABC12345", "PDF Document", "1.5 MB", []zotero.Tag{
			{Tag: "org:escherichia-coli"},
			{Tag: "topic:biofilm"},
		})
		out.RenderSummary(10, 8, 1, 1)
	})
}

func TestOutput_ConcurrentCalls(t *testing.T) {
	out := NewOutput()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			out.RenderItemResult("Concurrent Paper", "KEY123", "PDF Document", "1 MB", []zotero.Tag{
				{Tag: "org:streptococcus-mutans"},
				{Tag: "topic:oral-microbiology"},
			})
		}(i)
	}

	wg.Wait()
}
