package tagging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseResponse_ValidJSON(t *testing.T) {
	raw := `{"org_tags": ["org:escherichia-coli"], "group_tags": ["group:enterobacteriaceae"], "topic_tags": ["topic:pcr"]}`
	result, err := ParseResponse(raw)

	assert.NoError(t, err)
	assert.Equal(t, []string{"org:escherichia-coli"}, result.OrgTags)
	assert.Equal(t, []string{"group:enterobacteriaceae"}, result.GroupTags)
	assert.Equal(t, []string{"topic:pcr"}, result.TopicTags)
}

func TestParseResponse_FencedJSON(t *testing.T) {
	raw := "```json\n{\"org_tags\": [\"org:escherichia-coli\"], \"group_tags\": [], \"topic_tags\": []}\n```"
	result, err := ParseResponse(raw)

	assert.NoError(t, err)
	assert.Equal(t, []string{"org:escherichia-coli"}, result.OrgTags)
}

func TestFormatTag(t *testing.T) {
	tag := FormatTag("Escherichia coli", "org:")
	assert.Equal(t, "org:escherichia-coli", tag)

	tag2 := FormatTag("org:streptococcus pneumoniae", "org:")
	assert.Equal(t, "org:streptococcus-pneumoniae", tag2)
}

func TestFilterControlledTopics(t *testing.T) {
	result := &TagResult{
		OrgTags:   []string{"org:escherichia-coli"},
		GroupTags: []string{"group:enterobacteriaceae"},
		TopicTags: []string{"topic:biofilm", "topic:hallucinated-topic", "topic:oral-microbiology"},
	}

	allowedTopics := []string{"biofilm", "oral microbiology", "pcr"}
	filtered := FilterControlledTopics(result, allowedTopics)

	assert.NotNil(t, filtered)
	assert.Equal(t, []string{"topic:biofilm", "topic:oral-microbiology"}, filtered.TopicTags)
	assert.Equal(t, []string{"org:escherichia-coli"}, filtered.OrgTags)
	assert.Equal(t, []string{"group:enterobacteriaceae"}, filtered.GroupTags)

	// Ensure original was not mutated
	assert.Len(t, result.TopicTags, 3)

	// Nil safety
	assert.Nil(t, FilterControlledTopics(nil, allowedTopics))
}

func TestBuildTagList(t *testing.T) {
	result := &TagResult{
		OrgTags:   []string{"org:escherichia-coli"},
		GroupTags: []string{"group:enterobacteriaceae"},
		TopicTags: []string{"topic:biofilm"},
	}

	tags := BuildTagList(result, "_ai-tagged")
	assert.Len(t, tags, 4)
	assert.Equal(t, "org:escherichia-coli", tags[0].Tag)
	assert.Equal(t, "group:enterobacteriaceae", tags[1].Tag)
	assert.Equal(t, "topic:biofilm", tags[2].Tag)
	assert.Equal(t, "_ai-tagged", tags[3].Tag)
}

func TestBuildSystemPrompt(t *testing.T) {
	topics := []string{"biofilm", "pcr"}
	prompt := BuildSystemPrompt(topics)
	assert.Contains(t, prompt, "biofilm, pcr")
}

func TestBuildUserPrompt(t *testing.T) {
	p1 := BuildUserPrompt("Test Title", "Sample abstract", []string{"tag1", "tag2"})
	assert.Contains(t, p1, "Title: Test Title")
	assert.Contains(t, p1, "Existing Tags: tag1, tag2")
	assert.Contains(t, p1, "Sample abstract")

	p2 := BuildUserPrompt("Test Title 2", "", nil)
	assert.Contains(t, p2, "Title: Test Title 2")
	assert.Contains(t, p2, "Existing Tags: none")
	assert.Contains(t, p2, "Please analyze the attached paper document.")
}
