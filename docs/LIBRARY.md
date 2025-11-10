# Model Library

The Model Library feature in modfetch provides a centralized view of all your downloaded AI models with rich metadata, search capabilities, and organization tools.

## Table of Contents

- [Overview](#overview)
- [Accessing the Library](#accessing-the-library)
- [Features](#features)
- [Navigation](#navigation)
- [Keyboard Shortcuts](#keyboard-shortcuts)
- [Filtering and Search](#filtering-and-search)
- [Model Details](#model-details)
- [Directory Scanning](#directory-scanning)
- [Metadata Sources](#metadata-sources)

## Overview

The Library tab provides a comprehensive view of all models in your collection, including:

- **Downloaded models** from HuggingFace, CivitAI, and direct URLs
- **Locally scanned models** discovered in your configured directories
- **Rich metadata** including model type, quantization, file size, and source
- **Search and filtering** to quickly find specific models
- **Favorites system** to mark important models
- **Detailed view** with specifications, descriptions, tags, and links

## Accessing the Library

There are three ways to access the Library:

1. **Tab Key**: Press `5` from any view
2. **Letter Key**: Press `L` (uppercase) from any view
3. **Mouse**: Click on the "Library" tab (if using mouse support)

The Library tab is the 6th tab in the TUI (after All, Pending, Active, Completed, Failed).

## Features

### Model List View

The main library view shows all your models in a scrollable list:

```
Model Library • Search: "llama" • Type: LLM • Source: huggingface

  llama-2-7b (LLM) • 3.8 GB • Q4_K_M • huggingface
▶ ★ mistral-7b-instruct (LLM) • 4.1 GB • Q5_K_S • huggingface
  sdxl-base-1.0 (Checkpoint) • 6.9 GB • FP16 • civitai

Showing 1-3 of 15 models | ↑↓ navigate • Enter view details • / search • F filter • Q quit
```

Each model entry displays:
- **Favorite indicator** (★) if marked as favorite
- **Model name** (truncated if too long)
- **Model type** (LLM, LoRA, VAE, Checkpoint, etc.)
- **File size** (human-readable format)
- **Quantization** (Q4_K_M, Q5_K_S, FP16, etc.)
- **Source** (color-coded: green for HuggingFace, pink for CivitAI, default for local)

### Model Detail View

Press `Enter` on any model to view full details:

```
llama-2-7b

Type: LLM
Version: v1.0
Source: huggingface
Author: Meta
License: MIT

Specifications
Architecture: Llama
Parameters: 7B
Quantization: Q4_K_M
Base Model: Llama-2

File Information
Size: 3.8 GB
Format: .gguf
Location: /home/user/models/llm/llama-2-7b.Q4_K_M.gguf

Description
A large language model for text generation, fine-tuned for instruction following
and conversation. Optimized for general-purpose tasks with 7 billion parameters.

Tags
llm, text-generation, instruction-following, meta

Usage Statistics
Downloads: 125,420
Times Used: 12
Last Used: 2025-11-10 14:30

User Data
Rating: ★★★★☆
★ Favorite
Notes: Great for coding tasks, fast inference

Links
Homepage: https://huggingface.co/meta/llama-2-7b
Repository: https://github.com/meta/llama-2

Press Esc to go back • F to toggle favorite • Q to quit
```

## Navigation

### In List View

- **`↑` / `k`**: Move selection up
- **`↓` / `j`**: Move selection down
- **`Page Up`**: Scroll up one page
- **`Page Down`**: Scroll down one page
- **`Home` / `g`**: Jump to first model
- **`End` / `G`**: Jump to last model
- **`Enter`**: View selected model details

### In Detail View

- **`Esc`**: Return to list view
- **`f`**: Toggle favorite status
- **`Q` / `q`**: Quit to main download view

### Pagination

The library automatically paginates when you have more models than fit on screen:

- The selected model (marked with `▶`) stays in view as you navigate
- The "Showing X-Y of Z models" footer indicates your position
- Smooth scrolling keeps context visible

## Keyboard Shortcuts

### Library-Specific

| Key | Action |
|-----|--------|
| `5` or `L` | Switch to Library tab |
| `Enter` | View model details |
| `Esc` | Back to list / Exit detail view |
| `f` | Toggle favorite on selected model |
| `/` | Activate search |
| `S` | Scan directories for models |
| `↑` `↓` `j` `k` | Navigate model list |
| `g` `G` | Jump to first/last model |

### Global Shortcuts

| Key | Action |
|-----|--------|
| `Q` `q` | Quit to downloads view |
| `?` | Show help screen |
| `1` `2` `3` `4` | Switch to download tabs |
| `6` or `M` | Switch to Settings |

## Filtering and Search

### Search

Press `/` to activate search mode:

1. Type your search query (searches model name, ID, description, tags)
2. Press `Enter` to apply search
3. Press `Esc` to cancel and clear search

Search is **case-insensitive** and matches partial strings.

**Example searches:**
- `llama` - Find all Llama models
- `7b` - Find models with 7B in name/description
- `Q4` - Find Q4 quantized models
- `lora` - Find all LoRA models

The active search is displayed in the header:
```
Model Library • Search: "llama"
```

### Filtering by Type

Filter models by type using the internal filter state (future enhancement will add UI controls):

**Supported types:**
- `LLM` - Large language models (.gguf, .ggml files)
- `LoRA` - Low-Rank Adaptation models
- `VAE` - Variational autoencoders
- `Checkpoint` - Full model checkpoints (.safetensors, .ckpt)
- `Embedding` - Text embeddings / textual inversions
- `ControlNet` - ControlNet models
- `Unknown` - Unrecognized model types

### Filtering by Source

Filter models by source:

**Supported sources:**
- `huggingface` - Models from HuggingFace Hub
- `civitai` - Models from CivitAI
- `local` - Locally scanned models
- `direct` - Direct URL downloads

### Favorites

Mark important models as favorites:

1. Select a model in list view or open detail view
2. Press `f` to toggle favorite status
3. Favorite models show a ★ indicator

Filter to show only favorites by enabling the favorites filter (future UI enhancement).

## Model Details

The detail view shows comprehensive information organized into sections:

### Basic Information
- Model name, ID, version
- Source platform
- Author and license

### Specifications
- Architecture (e.g., Llama, SDXL, GPT)
- Parameter count (7B, 13B, 70B, etc.)
- Quantization method (Q4_K_M, FP16, INT8, etc.)
- Base model (if fine-tuned)

### File Information
- File size (human-readable)
- File format (.gguf, .safetensors, .ckpt, etc.)
- Full path to file on disk

### Description
- Full model description (up to 500 characters)
- Truncated with ellipsis if longer

### Tags
- Comma-separated list of tags
- Useful for categorization and search

### Usage Statistics
- Download count (from source platform)
- Times used locally
- Last used timestamp

### User Data
- Star rating (1-5 stars: ★★★★★)
- Favorite status
- Personal notes

### Links
- Homepage URL
- Repository URL
- Documentation URL (if available)
- Author profile URL

## Directory Scanning

The scanner automatically discovers models in your configured directories.

### Triggering a Scan

Press `S` in the Library tab to scan directories:

```
Scanning directories...
Found 42 models (15 new, 27 existing)
```

### What Gets Scanned

The scanner searches these locations:

1. **Download Root**: `cfg.General.DownloadRoot`
2. **Placement Directories**: All `cfg.Placement.Rules[].Dest` paths

### Supported File Types

The scanner recognizes these model file extensions:

- **GGUF/GGML**: `.gguf`, `.ggml` (LLM models)
- **SafeTensors**: `.safetensors` (Stable Diffusion, LoRAs)
- **Checkpoints**: `.ckpt` (legacy Stable Diffusion)
- **PyTorch**: `.pt`, `.pth` (PyTorch models)
- **Binary**: `.bin` (various formats)
- **TensorFlow**: `.h5`, `.pb` (Keras/TF models)
- **ONNX**: `.onnx` (ONNX models)

### Metadata Extraction

For each discovered file, the scanner automatically extracts:

**From filename:**
- **Model name**: First component before delimiter
- **Quantization**: Pattern matching (Q4_K_M, Q5_K_S, FP16, etc.)
- **Parameter count**: Size patterns (7B, 13B, 70B)
- **Version**: Version strings (v1.0, v2, etc.)

**From file system:**
- **File size**: Actual file size in bytes
- **File format**: Extension
- **File path**: Full absolute path

**Inferred:**
- **Model type**: Based on filename and extension
  - `.gguf`/`.ggml` → LLM
  - `*lora*` → LoRA
  - `*vae*` → VAE
  - `*embedding*` → Embedding
  - `*controlnet*` → ControlNet
  - `.safetensors`/`.ckpt` → Checkpoint (default)

### Duplicate Detection

The scanner uses **indexed database queries** for fast duplicate detection:

- **O(log n) performance** via `idx_metadata_dest` index
- Files are **skipped if already in database** (by path)
- **No re-scanning** of existing models
- **Efficient for large libraries** (1000+ models)

### Scan Results

After scanning, you'll see a toast notification:

```
Scan complete: 15 new, 27 skipped, 0 errors
```

The library view automatically refreshes to show newly discovered models.

## Metadata Sources

modfetch can fetch rich metadata from multiple sources:

### HuggingFace Hub

For models downloaded from HuggingFace:

**API Endpoint:** `https://huggingface.co/api/models/{model_id}`

**Fetched metadata:**
- Model description
- Tags (text-generation, llm, etc.)
- License information
- Author username
- Download count
- Repository URL
- Model card thumbnail

**Authentication:**
- Set `HF_TOKEN` environment variable for private/gated models
- Token checked in Settings tab

### CivitAI

For models downloaded from CivitAI:

**API Endpoint:** `https://civitai.com/api/v1/models/{model_id}`

**Fetched metadata:**
- Model description
- Model type (Checkpoint, LoRA, etc.)
- Base model information
- Tags
- Download count
- Author information
- Preview images

**Authentication:**
- Set `CIVITAI_TOKEN` environment variable for NSFW/restricted content
- Token checked in Settings tab

### Direct Downloads

For direct URL downloads (non-HuggingFace, non-CivitAI):

**Basic metadata only:**
- URL as source
- Filename-based inference
- File size and format
- Source: "direct"

### Local Scans

For locally discovered models:

**Scanner-extracted metadata:**
- Filename-based extraction
- File system information
- Type inference from extension
- Source: "local"

**Enrichment:**
- If file was previously downloaded via modfetch, existing metadata is preserved
- If file came from external sources, only basic metadata is available

## Best Practices

### Organization

1. **Use favorites** for frequently accessed models
2. **Add user notes** to remember model quirks or use cases
3. **Rate models** to track quality (1-5 stars)
4. **Regular scans** after downloading models externally

### Search Tips

1. **Search by quantization**: `Q4_K_M` finds all Q4_K_M quantized models
2. **Search by size**: `7B` or `13B` finds models by parameter count
3. **Search by task**: `instruct`, `chat`, `code` to find specialized models
4. **Search by author**: Author name to find all models from a creator

### Metadata Enrichment

For best metadata:

1. **Download via modfetch** to get automatic metadata fetching
2. **Set API tokens** in environment for private model access:
   ```bash
   export HF_TOKEN="your_token_here"
   export CIVITAI_TOKEN="your_token_here"
   ```
3. **Check Settings tab** to verify token status

### Performance

For large libraries (1000+ models):

1. Scanner uses **indexed queries** - scales well
2. Pagination keeps UI responsive
3. Search is fast (LIKE query with indexes)
4. Consider **filtering by type/source** to narrow results

## Troubleshooting

### Models Not Appearing

**Problem:** Downloaded models don't show up in library

**Solutions:**
1. Press `S` to trigger a manual scan
2. Check that files are in configured directories:
   - `cfg.General.DownloadRoot`
   - `cfg.Placement.Rules[].Dest`
3. Verify file extension is supported (see [Supported File Types](#supported-file-types))

### Missing Metadata

**Problem:** Models show minimal information

**Solutions:**
1. **For HuggingFace models:**
   - Set `HF_TOKEN` if model is private/gated
   - Check API status at https://status.huggingface.co
   - Verify URL format is correct

2. **For CivitAI models:**
   - Set `CIVITAI_TOKEN` for restricted content
   - Check API status
   - Ensure model ID is valid

3. **For local scans:**
   - Metadata is limited to filename parsing
   - Rename files to include quantization (e.g., `model.Q4_K_M.gguf`)
   - Include parameter count in filename (e.g., `llama-7b.gguf`)

### Slow Scanning

**Problem:** Directory scan takes a long time

**Solutions:**
1. **Expected for first scan** of large directories
2. **Subsequent scans are fast** (duplicate detection via index)
3. **Reduce scope**: Remove unnecessary directories from placement rules
4. **Check for slow storage**: Network drives may be slow

### Search Not Finding Models

**Problem:** Search doesn't return expected results

**Solutions:**
1. **Check spelling**: Search is case-insensitive but requires correct spelling
2. **Try partial matches**: Search "llama" instead of "llama-2-7b"
3. **Check search fields**: Searches name, ID, description, tags (not file path)
4. **Clear filters**: Active type/source filters may exclude results

## Advanced Usage

### Custom Metadata Fields

Add personal notes and ratings:

1. Open model detail view
2. Notes and ratings are stored per-model
3. Searchable via the main search function

### Bulk Operations (Future Enhancement)

Planned features:
- Bulk favorite/unfavorite
- Bulk tagging
- Export library to CSV/JSON
- Import from external catalogs

### Integration with Download Manager

The library is integrated with the download system:

1. **Automatic metadata fetch** during download
2. **Progress tracking** shows in download tabs
3. **Completion triggers** library refresh
4. **Metadata stored** in SQLite database

## Configuration

Library behavior is controlled by config file settings:

```yaml
general:
  download_root: /home/user/models  # Scanned for models

placement:
  rules:
    - dest: /opt/comfyui/models     # Also scanned

sources:
  huggingface:
    enabled: true
    token_env: HF_TOKEN             # For private models

  civitai:
    enabled: true
    token_env: CIVITAI_TOKEN        # For NSFW/restricted

ui:
  refresh_hz: 10                    # Library refresh rate
```

## See Also

- [Scanner Documentation](SCANNER.md) - Detailed scanner internals
- [TUI Guide](TUI_GUIDE.md) - Full TUI documentation
- [Configuration Guide](CONFIG.md) - Config file reference
- [Metadata API](METADATA.md) - Metadata system details
