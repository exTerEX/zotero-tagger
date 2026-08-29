package tagging

import (
	"fmt"
	"strings"
)

const SystemPrompt = `You are an expert microbiological taxonomy and topic classifier for academic papers.

Analyze the provided paper document or text, title, and existing tags.
Identify the specific organisms (org:), broader taxonomic clades or traits (group:), and 2-5 controlled topics (topic:) from the provided list that represent the CORE SCIENTIFIC FOCUS of the paper.

RULES:
1. 'org:' tags MUST be lowercase binomial species names with hyphens (e.g. 'org:actinobacillus-actinomycetemcomitans', 'org:streptococcus-pneumoniae').
2. 'group:' tags represent broader taxonomic clades, families, or phenotypic traits (e.g. 'group:pasteurellaceae', 'group:gram-negative', 'group:mycobacteria', 'group:fungi').
3. 'topic:' tags MUST be strictly chosen from this controlled list ONLY: {CONTROLLED_TOPICS}. Do not invent new topics.
4. CRITICAL ORGANISM SELECTION RULE: Only tag organisms ('org:') or groups ('group:') that are the PRIMARY SUBJECT or CORE FOCUS of the research paper. DO NOT tag model organisms, expression hosts, laboratory helper strains, or cloning vectors (such as Escherichia coli, Saccharomyces cerevisiae, or phages) IF THEY ARE ONLY USED AS CLONING HOSTS, EXPRESSION SYSTEMS, OR ROUTINE METHODOLOGY TOOLS in the experiments.

Respond STRICTLY in JSON:
{
  "org_tags": ["org:genus-species"],
  "group_tags": ["group:clade-or-trait"],
  "topic_tags": ["topic:controlled-term"]
}`

func BuildSystemPrompt(controlledTopics []string) string {
	topicsStr := strings.Join(controlledTopics, ", ")
	return strings.Replace(SystemPrompt, "{CONTROLLED_TOPICS}", topicsStr, 1)
}

func BuildUserPrompt(title, extraText string, existingTags []string) string {
	tagsStr := strings.Join(existingTags, ", ")
	if tagsStr == "" {
		tagsStr = "none"
	}

	if strings.TrimSpace(extraText) != "" {
		return fmt.Sprintf("Title: %s\nExisting Tags: %s\nAbstract/Context:\n%s", title, tagsStr, extraText)
	}
	return fmt.Sprintf("Title: %s\nExisting Tags: %s\nPlease analyze the attached paper document.", title, tagsStr)
}
