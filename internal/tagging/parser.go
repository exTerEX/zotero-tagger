package tagging

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type TagResult struct {
	OrgTags   []string `json:"org_tags"`
	GroupTags []string `json:"group_tags"`
	TopicTags []string `json:"topic_tags"`
}

func ParseResponse(raw string) (*TagResult, error) {
	var result TagResult
	if err := json.Unmarshal([]byte(raw), &result); err == nil {
		return &result, nil
	}

	fenceRe := regexp.MustCompile("(?s)```(?:json)?\\s*(.+?)\\s*```")
	if matches := fenceRe.FindStringSubmatch(raw); len(matches) > 1 {
		if err := json.Unmarshal([]byte(matches[1]), &result); err == nil {
			return &result, nil
		}
	}

	return nil, fmt.Errorf("failed to parse JSON from LLM response")
}

func FilterControlledTopics(result *TagResult, allowedTopics []string) *TagResult {
	if result == nil {
		return nil
	}

	allowedMap := make(map[string]bool)
	for _, t := range allowedTopics {
		allowedMap[strings.ToLower(strings.TrimSpace(t))] = true
	}

	var validTopics []string
	for _, topic := range result.TopicTags {
		cleanTopic := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(topic)), "topic:")
		cleanTopic = strings.ReplaceAll(cleanTopic, "-", " ")
		if allowedMap[cleanTopic] {
			validTopics = append(validTopics, topic)
		}
	}

	return &TagResult{
		OrgTags:   append([]string(nil), result.OrgTags...),
		GroupTags: append([]string(nil), result.GroupTags...),
		TopicTags: validTopics,
	}
}
