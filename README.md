# Zotero AI Tagger

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Google Gemini](https://img.shields.io/badge/LLM-Google%20Gemini-4285F4?style=flat&logo=google)](https://ai.google.dev)
[![Docker Ready](https://img.shields.io/badge/Docker-Containerized-2496ED?style=flat&logo=docker)](docker/Dockerfile)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

An automated, high-performance CLI tool that categorizes and tags academic literature in **Zotero** libraries using Google Gemini large language models.

By uploading PDF documents directly to Google via the Google GenAI SDK (defaulting to `gemini-3.5-flash-lite`), `zotero-tagger` delivers standardized, hierarchical, and controlled taxonomy tags back into your Zotero database with full idempotency and concurrency.

---

## Key Features

- **Direct Zotero REST API Sync**: Seamlessly processes user libraries or shared group libraries with automatic pagination, exponential backoff, and HTTP 412 optimistic locking prevention.
- **Native PDF Document Understanding**: Directly uploads attached PDF files to Google via the Google GenAI Files API for multimodal comprehension—no local `pdftotext` or `poppler-utils` required!
- **Pure Multimodal Taxonomy Extraction**:
  - Leverages Google Gemini's advanced multimodal reasoning to analyze the complete paper and identify focal organisms, higher-order clades, and topic tags.
- **Strict, Multi-Tiered Taxonomy Namespacing**:
  - `org:` &mdash; Standardized lowercase binomial species names (e.g., `org:streptococcus-mutans`). Excludes lab-tool cloning hosts and expression vectors unless they are the primary subject of research.
  - `group:` &mdash; Higher-level taxonomic clades, families, or phenotypic traits (e.g., `group:streptococcaceae`, `group:gram-negative`).
  - `topic:` &mdash; Controlled subject terms validated strictly against a user-defined vocabulary list to prevent taxonomy drift and hallucinated keywords.
- **Google GenAI Integration**: Uses the official Google GenAI SDK (`google.golang.org/genai`) defaulting to high-speed, cost-effective `gemini-3.5-flash-lite`.
- **Safety, Concurrency & Idempotency**:
  - Sentinel tagging (`_ai-tagged`) prevents duplicate processing on subsequent runs.
  - `--dry-run` mode provides rich terminal table previews without mutating your Zotero library.
  - Built-in multi-worker concurrency and token-aware rate limiting (RPM, TPM, RPD).
  - Local disk cache (`--cache`) to eliminate redundant LLM API calls during testing.
- **Container Ready**: Includes a multi-stage Docker build and Docker Compose configuration.

---

## Workflow Architecture

```mermaid
flowchart LR
    A[Zotero Library] -->|Fetch Unprocessed Items| B[PDF / Abstract Receiver]
    B -->|Upload Raw PDF to Google| C[Google Gemini API]
    C --> D[JSON Schema & Taxonomy Validator]
    D -->|Controlled Topic Filter| E[Tag Builder]
    E -->|Optimistic Lock Update| A
```

---

## Installation & Setup

### Prerequisites

- **Go 1.23+** (if building from source)
- **Zotero Account & API Key**
- **Google Gemini API Key** (`GEMINI_API_KEY`)

### 1. Clone & Build

Using Make:

```bash
git clone https://github.com/andreassag/zotero-tagger.git
cd zotero-tagger
make build
```

Or using standard Go CLI:

```bash
go build -o zotero-tagger ./cmd/zotero-tagger
```

### 2. Configure Environment Variables

Copy the example environment template:

```bash
cp .env.example .env
```

Edit `.env` with your API credentials:

```env
# Zotero API credentials
# Obtain your User ID and generate an API key at https://www.zotero.org/settings/keys
ZOTERO_USER_ID=12345678
ZOTERO_API_KEY=your_zotero_api_key

# Gemini API credentials
# Obtain your API key from Google AI Studio at https://aistudio.google.com/app/apikey
GEMINI_API_KEY=your_gemini_api_key
```

### 3. Customize `config/config.toml`

Adjust the configuration file to match your model, rate limits, and controlled topics:

```toml
[llm]
model_name = "gemini-3.5-flash-lite"
temperature = 0.0

[llm.rate_limits]
requests_per_minute = 1000
tokens_per_minute = 1000000
requests_per_day = 10000

[zotero]
library_type = "user" # "user" or "group"

[tagging.controlled_topics]
topics = [
    "oral microbiology",
    "biofilm",
    "quorum-sensing",
    "antimicrobial resistance",
    "genomics",
    "metagenomics",
    "microbiome"
]
```

### 4. Git Hooks Configuration

Enable the repository's automated pre-commit quality checks:

```bash
git config core.hooksPath .githooks
```

The pre-commit hook automatically validates:
- Code formatting (`gofmt -s`)
- Static analysis (`go vet`)
- Linting (`golangci-lint` if installed)
- Unit test suite (`go test -short`)

---

## Usage

### Command-Line Reference

```bash
zotero-tagger tag [flags]
```

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--config` | `string` | `config/config.toml` | Path to TOML configuration file |
| `--dry-run` | `bool` | `false` | Preview generated tags without modifying Zotero items |
| `--limit` | `int` | `0` | Maximum number of items to process (`0` = all untagged) |
| `--collection` | `string` | `""` | Filter processing to a specific Zotero collection key |
| `--group` | `string` | `""` | Process a specific Zotero Group Library ID |
| `--item` | `string` | `""` | Process a single specific Zotero item key |
| `--concurrent` | `int` | `1` | Number of parallel worker threads |
| `--reprocess` | `bool` | `false` | Reprocess items even if the sentinel tag (`_ai-tagged`) exists |
| `--verbose` | `bool` | `false` | Enable detailed debug logging |
| `--json-log` | `bool` | `false` | Output logs in structured JSON format |
| `--cache` | `bool` | `false` | Cache LLM prompt responses on disk to prevent redundant API calls |
| `--skip-llm` | `bool` | `false` | Skip LLM calls and Zotero tag updates to inspect text extraction only |

---

### Examples

#### 1. Dry Run / Preview

Run a dry run on 5 items to preview generated tags in the terminal:

```bash
./zotero-tagger tag --dry-run --limit 5
```

#### 2. Process a Specific Item

Tag a single library item by its Zotero key:

```bash
./zotero-tagger tag --item ITEM_KEY
```

#### 3. Process a Collection with Concurrency

Process items within a specific collection using 4 concurrent workers:

```bash
./zotero-tagger tag --collection COLLECTION_KEY --concurrent 4
```

#### 4. Process a Shared Group Library

Process papers in a shared Zotero group library:

```bash
./zotero-tagger tag --group GROUP_ID
```

#### 5. Force Reprocessing of Previously Tagged Items

Refresh tags across all papers, bypassing the sentinel tag:

```bash
./zotero-tagger tag --reprocess
```

#### 6. Development & Inspection Mode

Test document acquisition without calling the LLM API:

```bash
./zotero-tagger tag --skip-llm --limit 3 --verbose
```

---

## Running with Docker

`zotero-tagger` provides a lightweight Alpine container image with CA certificates pre-configured.

### Using Docker Compose

1. Configure your `.env` and `config/config.toml` files.
2. Launch the containerized pipeline:

```bash
docker compose -f docker/docker-compose.yml up --build
```

### Using Docker CLI

```bash
# Build the image
docker build -t zotero-tagger -f docker/Dockerfile .

# Run dry run with local configuration mounted
docker run --rm \
  --env-file .env \
  -v $(pwd)/config/config.toml:/etc/zotero-tagger/config.toml:ro \
  zotero-tagger tag --dry-run --limit 5
```

---

## Detailed Documentation

For full architectural specifications, taxonomy schemas, model configuration, and troubleshooting guides, see the [Detailed Documentation Guide](docs/README.md).

---

## License

This project is licensed under the [MIT License](LICENSE).
